import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { LockIcon, PlusIcon, ReceiptIcon, Trash2Icon, WalletIcon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
  createExpenseMutation,
  deleteExpenseMutation,
  listOutlaysOptions,
  updateExpenseMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import type { OutlayListItem } from "@/api/gen/types.gen";
import { costTypeLabel, IssueCounts, Money, PageHeader } from "@/components/common";
import { ExpenseForm } from "@/components/expense-form";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { EmptyState, Skeleton } from "@/components/ui/misc";
import { formatDate, formatMinor } from "@/lib/format";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/outlays/")({
  validateSearch: (s: Record<string, unknown>): { new?: boolean } => ({
    new: s.new === true || s.new === "true" ? true : undefined,
  }),
  component: Outlays,
});

function Outlays() {
  const search = Route.useSearch();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery(listOutlaysOptions());
  const [editing, setEditing] = useState<OutlayListItem | "new" | null>(search.new ? "new" : null);
  const close = () => {
    setEditing(null);
    if (search.new) navigate({ to: "/outlays", search: {}, replace: true });
  };
  const refresh = () => {
    qc.invalidateQueries({ queryKey: [{ _id: "listOutlays" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "listReceipts" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "getDashboard" }] });
  };
  const done = () => {
    refresh();
    close();
    toast.success(m.expense_saved());
  };
  const create = useMutation({ ...createExpenseMutation(), onSuccess: done });
  const update = useMutation({ ...updateExpenseMutation(), onSuccess: done });
  const del = useMutation({
    ...deleteExpenseMutation(),
    onSuccess: () => {
      refresh();
      close();
    },
  });
  const newButton = (
    <Button onClick={() => setEditing("new")}>
      <PlusIcon />
      {m.new_outlay()}
    </Button>
  );
  return (
    <>
      <PageHeader title={m.nav_outlays()} description={m.outlays_subtitle()} actions={newButton} />
      {isLoading ? (
        <Skeleton className="h-40" />
      ) : !data?.length ? (
        <EmptyState icon={WalletIcon} title={m.no_outlays()} text={m.no_outlays_text()} action={newButton} />
      ) : (
        <Card className="divide-y overflow-hidden">
          {data.map((o) => (
            <button
              key={o.id}
              type="button"
              onClick={() => setEditing(o)}
              disabled={o.locked}
              className="flex w-full items-center gap-3 px-4 py-3.5 text-left hover:bg-accent/60 disabled:hover:bg-transparent sm:px-5"
            >
              <div className="grid min-w-0 flex-1 gap-1">
                <span className="truncate font-medium">{o.description || o.supplier}</span>
                <span className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                  {formatDate(o.date)} · {costTypeLabel(o.costType)}
                  {o.supplier && o.description ? ` · ${o.supplier}` : ""}
                  {o.locked ? (
                    <Badge variant="outline">
                      <LockIcon className="size-3" />
                      {m.locked()}
                    </Badge>
                  ) : o.claimId ? (
                    <Badge variant="outline">{m.in_claim()}</Badge>
                  ) : (
                    <Badge>{m.not_claimed()}</Badge>
                  )}
                  {o.receipts.length ? (
                    <Badge variant="default">
                      <ReceiptIcon className="size-3" />
                      {o.receipts.map((r) => r.number || m.receipt_unconfirmed()).join(", ")}
                    </Badge>
                  ) : null}
                  <IssueCounts
                    blockers={o.issues.filter((i) => i.severity === "blocker").length}
                    warnings={o.issues.filter((i) => i.severity === "warning").length}
                  />
                </span>
              </div>
              <div className="grid text-right">
                <Money cents={o.result.reimbursable} strong />
                {o.currency !== "EUR" ? (
                  <span className="text-xs text-muted-foreground">
                    {formatMinor(o.grossMinor, o.currency)}
                  </span>
                ) : null}
              </div>
            </button>
          ))}
        </Card>
      )}
      <Dialog open={editing !== null} onOpenChange={(o) => !o && close()}>
        {editing !== null ? (
          <DialogContent
            title={editing === "new" ? m.new_outlay() : m.edit_outlay()}
            description={m.outlay_dialog_hint()}
            wide
          >
            <ExpenseForm
              expense={editing === "new" ? undefined : editing}
              pending={create.isPending || update.isPending}
              onSubmit={(body) =>
                editing === "new"
                  ? create.mutate({ body })
                  : update.mutate({ path: { id: editing.id }, body })
              }
            />
            {editing !== "new" && !editing.claimId ? (
              <Button
                variant="ghost"
                className="mt-2 w-full text-destructive"
                onClick={() =>
                  confirm(m.confirm_delete_expense()) && del.mutate({ path: { id: editing.id } })
                }
              >
                <Trash2Icon />
                {m.delete_expense()}
              </Button>
            ) : null}
          </DialogContent>
        ) : null}
      </Dialog>
    </>
  );
}
