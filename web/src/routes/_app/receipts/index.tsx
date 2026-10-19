import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  BanIcon,
  CheckIcon,
  CopyIcon,
  ExternalLinkIcon,
  FileIcon,
  InboxIcon,
  ReceiptIcon,
  Trash2Icon,
  UploadIcon,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
  confirmReceiptMutation,
  deleteReceiptMutation,
  listReceiptsOptions,
  voidReceiptMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import type { Receipt } from "@/api/gen/types.gen";
import { PageHeader } from "@/components/common";
import { ReceiptCapture, ReceiptFilePicker, useUploadReceipts } from "@/components/receipt-capture";
import { SelfIssuedDialog } from "@/components/self-issued";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Alert, EmptyState, Skeleton } from "@/components/ui/misc";
import { Tabs, TabsList, TabsPanel, TabsTab } from "@/components/ui/tabs";
import { formatDate, formatWeekday } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/receipts/")({
  validateSearch: (s: Record<string, unknown>): { focus?: number } => ({
    focus: s.focus ? Number(s.focus) : undefined,
  }),
  component: Receipts,
});

function Receipts() {
  const { focus } = Route.useSearch();
  const navigate = useNavigate();
  const { data = [], isLoading } = useQuery(listReceiptsOptions({ query: {} }));
  const upload = useUploadReceipts();
  const [dragging, setDragging] = useState(false);
  const inbox = data.filter((r) => r.status === "inbox");
  const confirmed = data.filter((r) => r.status === "confirmed");
  const voided = data.filter((r) => r.status === "void");
  const open = data.find((r) => r.id === focus);
  const setOpen = (r?: Receipt) =>
    navigate({ to: "/receipts", search: r ? { focus: r.id } : {}, replace: true });

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: drop zone; keyboard users use the upload buttons
    <div
      onDragOver={(e) => {
        e.preventDefault();
        setDragging(true);
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={(e) => {
        e.preventDefault();
        setDragging(false);
        if (e.dataTransfer.files.length) upload.mutate(Array.from(e.dataTransfer.files));
      }}
      className={cn(
        "rounded-xl transition-colors",
        dragging && "bg-accent/50 outline-2 outline-dashed outline-primary",
      )}
    >
      <PageHeader
        title={m.nav_receipts()}
        description={m.receipts_subtitle()}
        actions={
          <>
            <ReceiptCapture
              onUploaded={(ids) => setOpen(data.find((r) => r.id === ids[0]) ?? ({ id: ids[0] } as Receipt))}
            />
            <ReceiptFilePicker />
            <SelfIssuedDialog />
          </>
        }
      />
      <p className="mb-4 hidden items-center gap-2 text-sm text-muted-foreground md:flex">
        <UploadIcon className="size-4" />
        {m.drop_hint()}
      </p>
      {isLoading ? (
        <Skeleton className="h-60" />
      ) : (
        <Tabs defaultValue={inbox.length || !confirmed.length ? "inbox" : "confirmed"}>
          <TabsList>
            <TabsTab value="inbox">
              {m.inbox()}
              {inbox.length ? <Badge variant="primary">{inbox.length}</Badge> : null}
            </TabsTab>
            <TabsTab value="confirmed">{m.confirmed()}</TabsTab>
            <TabsTab value="void">{m.voided()}</TabsTab>
          </TabsList>
          <TabsPanel value="inbox" className="pt-4">
            {inbox.length ? (
              <Grid receipts={inbox} onOpen={setOpen} />
            ) : (
              <EmptyState icon={InboxIcon} title={m.inbox_empty()} text={m.inbox_empty_text()} />
            )}
          </TabsPanel>
          <TabsPanel value="confirmed" className="pt-4">
            {confirmed.length ? (
              <Grid receipts={confirmed} onOpen={setOpen} />
            ) : (
              <EmptyState icon={ReceiptIcon} title={m.no_confirmed()} />
            )}
          </TabsPanel>
          <TabsPanel value="void" className="pt-4">
            {voided.length ? (
              <Grid receipts={voided} onOpen={setOpen} />
            ) : (
              <p className="text-sm text-muted-foreground">{m.no_voided()}</p>
            )}
          </TabsPanel>
        </Tabs>
      )}
      <Dialog open={!!focus} onOpenChange={(o) => !o && setOpen()}>
        {focus ? <ReceiptDialog receipt={open} onClose={() => setOpen()} /> : null}
      </Dialog>
    </div>
  );
}

function Grid({ receipts, onOpen }: { receipts: Receipt[]; onOpen: (r: Receipt) => void }) {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {receipts.map((r) => (
        <button
          key={r.id}
          type="button"
          onClick={() => onOpen(r)}
          className="group overflow-hidden rounded-xl border bg-card text-left shadow-xs transition hover:shadow-md focus-visible:ring-2 focus-visible:ring-ring"
        >
          <div className="aspect-[3/4] overflow-hidden bg-muted">
            {r.hasPreview ? (
              <img
                src={`/api/receipts/${r.id}/preview`}
                alt=""
                loading="lazy"
                className="size-full object-cover object-top transition group-hover:scale-[1.02]"
              />
            ) : (
              <div className="grid size-full place-items-center text-muted-foreground">
                <FileIcon className="size-8" />
              </div>
            )}
          </div>
          <div className="grid gap-1 p-2.5">
            <span className="truncate text-sm font-medium">{r.number || r.filename}</span>
            <span className="flex flex-wrap gap-1 text-xs text-muted-foreground">
              {formatDate(r.createdAt)}
              {r.pages > 1 ? ` · ${m.n_pages({ count: String(r.pages) })}` : ""}
            </span>
            <span className="flex flex-wrap gap-1">
              {r.duplicate ? (
                <Badge variant="warning">
                  <CopyIcon className="size-3" />
                  {m.duplicate()}
                </Badge>
              ) : null}
              {r.status === "inbox" ? <Badge variant="warning">{m.to_confirm()}</Badge> : null}
              {r.status !== "void" && r.expenses.length === 0 ? (
                <Badge variant="outline">{m.unassigned()}</Badge>
              ) : null}
            </span>
          </div>
        </button>
      ))}
    </div>
  );
}

function ReceiptDialog({ receipt, onClose }: { receipt?: Receipt; onClose: () => void }) {
  const qc = useQueryClient();
  const [reason, setReason] = useState("");
  const refresh = () => {
    qc.invalidateQueries({ queryKey: [{ _id: "listReceipts" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "getDashboard" }] });
  };
  const confirmR = useMutation({
    ...confirmReceiptMutation(),
    onSuccess: (r) => {
      refresh();
      toast.success(m.receipt_confirmed({ number: r.number }));
    },
  });
  const del = useMutation({
    ...deleteReceiptMutation(),
    onSuccess: () => {
      refresh();
      onClose();
    },
  });
  const voidR = useMutation({
    ...voidReceiptMutation(),
    onSuccess: () => {
      refresh();
      onClose();
    },
  });
  if (!receipt?.status) {
    return (
      <DialogContent title={m.receipt()}>
        <Skeleton className="h-80" />
      </DialogContent>
    );
  }
  const r = receipt;
  const isPDF = r.mime === "application/pdf";
  return (
    <DialogContent
      title={r.number ? `${m.receipt()} ${r.number}` : m.check_receipt()}
      description={r.filename}
      wide
    >
      <div className="grid gap-4 md:grid-cols-[1fr_16rem]">
        <div className="grid max-h-[60dvh] place-items-center overflow-auto rounded-lg border bg-muted">
          {r.kind === "self_issued" && r.selfIssued ? (
            <div className="grid w-full gap-2 bg-card p-4 text-sm">
              <p className="font-semibold">{m.self_issued()}</p>
              <p>
                {formatDate(r.selfIssued.date)} · {(r.selfIssued.amountMinor / 100).toFixed(2)}{" "}
                {r.selfIssued.currency}
              </p>
              <p>
                {r.selfIssued.payee} – {r.selfIssued.purpose}
              </p>
              <p className="text-muted-foreground">{r.selfIssued.reason}</p>
            </div>
          ) : isPDF ? (
            <img src={`/api/receipts/${r.id}/preview`} alt={m.receipt_preview()} className="w-full" />
          ) : (
            <img
              src={`/api/receipts/${r.id}/file`}
              alt={m.receipt_preview()}
              className="max-h-[60dvh] w-auto object-contain"
            />
          )}
        </div>
        <div className="grid content-start gap-3">
          {r.kind === "file" ? (
            <a
              href={`/api/receipts/${r.id}/file`}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 text-sm text-primary hover:underline"
            >
              <ExternalLinkIcon className="size-4" />
              {isPDF ? m.open_pdf({ pages: String(r.pages) }) : m.open_original()}
            </a>
          ) : null}
          {r.duplicate ? <Alert variant="warning">{m.duplicate_text()}</Alert> : null}
          {r.status === "inbox" ? (
            <>
              <p className="text-sm text-muted-foreground">{m.confirm_receipt_text()}</p>
              <Button onClick={() => confirmR.mutate({ path: { id: r.id } })} disabled={confirmR.isPending}>
                <CheckIcon />
                {m.confirm_receipt()}
              </Button>
              <Button
                variant="ghost"
                className="text-destructive"
                onClick={() => del.mutate({ path: { id: r.id } })}
                disabled={r.expenses.length > 0}
              >
                <Trash2Icon />
                {m.delete_upload()}
              </Button>
            </>
          ) : null}
          {r.expenses.length ? (
            <div className="grid gap-1">
              <p className="text-sm font-medium">{m.linked_to()}</p>
              {r.expenses.map((e) =>
                e.tripId ? (
                  <Link
                    key={e.id}
                    to="/trips/$tripId"
                    params={{ tripId: String(e.tripId) }}
                    className="text-sm text-primary hover:underline"
                  >
                    {formatWeekday(e.date)} · {e.supplier || e.description}
                  </Link>
                ) : (
                  <Link key={e.id} to="/outlays" className="text-sm text-primary hover:underline">
                    {formatWeekday(e.date)} · {e.description || e.supplier}
                  </Link>
                ),
              )}
            </div>
          ) : r.status !== "void" ? (
            <p className="text-sm text-muted-foreground">{m.receipt_assign_hint()}</p>
          ) : null}
          {r.status === "confirmed" ? (
            <details className="text-sm">
              <summary className="cursor-pointer text-muted-foreground">{m.void_receipt()}</summary>
              <div className="mt-2 grid gap-2">
                <Input placeholder={m.reason()} value={reason} onChange={(e) => setReason(e.target.value)} />
                <Button
                  variant="outline"
                  disabled={!reason || voidR.isPending}
                  onClick={() => voidR.mutate({ path: { id: r.id }, body: { reason } })}
                >
                  <BanIcon />
                  {m.void_receipt()}
                </Button>
              </div>
            </details>
          ) : null}
          {r.status === "void" ? <Alert>{m.voided_reason({ reason: r.voidReason ?? "" })}</Alert> : null}
          <p className="break-all font-mono text-[10px] text-muted-foreground">SHA-256 {r.sha256}</p>
        </div>
      </div>
    </DialogContent>
  );
}
