import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  ArrowLeftIcon,
  BanknoteIcon,
  CheckCircle2Icon,
  DownloadIcon,
  FileArchiveIcon,
  FileTextIcon,
  Loader2Icon,
  LockOpenIcon,
  MinusIcon,
  PlusIcon,
  SendIcon,
  Trash2Icon,
  Undo2Icon,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
  deleteClaimMutation,
  getClaimOptions,
  markClaimPaidMutation,
  setClaimItemMutation,
  submitClaimMutation,
  undoClaimPaidMutation,
  unlockClaimMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import type { Preview } from "@/api/gen/types.gen";
import { ClaimStatusBadge, costTypeLabel, IssueList, Money, PageHeader, Row } from "@/components/common";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Checkbox, Field, Input, Textarea } from "@/components/ui/input";
import { Alert, Skeleton } from "@/components/ui/misc";
import { errorText } from "@/lib/api";
import { formatDate, formatRange, todayISO } from "@/lib/format";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/claims/$claimId")({ component: ClaimPage });

function ClaimPage() {
  const { claimId } = Route.useParams();
  const id = Number(claimId);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const query = getClaimOptions({ path: { id } });
  const { data: p, isLoading, error } = useQuery(query);
  const refresh = () => {
    qc.invalidateQueries({ queryKey: query.queryKey });
    qc.invalidateQueries({ queryKey: [{ _id: "listClaims" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "listTrips" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "listOutlays" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "listAdvances" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "getDashboard" }] });
  };
  const item = useMutation({ ...setClaimItemMutation(), onSuccess: refresh });
  const del = useMutation({
    ...deleteClaimMutation(),
    onSuccess: () => {
      refresh();
      navigate({ to: "/claims" });
    },
  });
  if (isLoading) return <Skeleton className="h-96" />;
  if (error || !p) return <Alert variant="error">{m.not_found()}</Alert>;
  const c = p.claim;
  const draft = c.status === "draft";
  const toggle = (kind: "trip" | "outlay" | "advance", itemId: number, include: boolean) =>
    item.mutate({ path: { id }, body: { kind, id: itemId, include } });

  return (
    <>
      <PageHeader
        back={
          <Link
            to="/claims"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          >
            <ArrowLeftIcon className="size-4" />
            {m.nav_claims()}
          </Link>
        }
        title={
          <span className="flex flex-wrap items-center gap-3">
            {c.title || c.number || m.claim_untitled()}
            <ClaimStatusBadge status={c.status} />
          </span>
        }
        description={`${formatDate(c.periodStart)} – ${formatDate(c.periodEnd)}${c.number ? ` · ${c.number}` : ""}${c.currentVersion ? ` · ${m.version_n({ n: String(c.currentVersion) })}` : ""}`}
        actions={
          draft && c.currentVersion === 0 ? (
            <Button
              variant="ghost"
              className="text-destructive"
              onClick={() => confirm(m.confirm_delete_claim()) && del.mutate({ path: { id } })}
            >
              <Trash2Icon />
              {m.delete()}
            </Button>
          ) : null
        }
      />

      <div className="grid gap-4 lg:grid-cols-[1fr_21rem]">
        <div className="grid min-w-0 gap-4">
          {draft ? <IssuesCard p={p} /> : null}

          <Card>
            <CardHeader>
              <CardTitle>{m.travel_reimbursement()}</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              {p.trips.length === 0 ? (
                <p className="text-sm text-muted-foreground">{m.no_trips_in_claim()}</p>
              ) : null}
              <ul className="divide-y">
                {p.trips.map((t) => (
                  <li key={t.trip.id} className="flex items-center gap-3 py-2.5">
                    <Link
                      to="/trips/$tripId"
                      params={{ tripId: String(t.trip.id) }}
                      className="grid min-w-0 flex-1 hover:underline"
                    >
                      <span className="truncate text-sm font-medium">{t.trip.purpose}</span>
                      <span className="text-xs text-muted-foreground">
                        {formatRange(t.trip.start, t.trip.end)}
                      </span>
                    </Link>
                    <Money cents={t.result.totals.total} strong />
                    {draft ? (
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={m.remove_from_claim()}
                        onClick={() => toggle("trip", t.trip.id, false)}
                      >
                        <MinusIcon />
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{m.standalone_outlays()}</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              {p.outlays.length === 0 ? (
                <p className="text-sm text-muted-foreground">{m.no_outlays_in_claim()}</p>
              ) : null}
              <ul className="divide-y">
                {p.outlays.map((o) => (
                  <li key={o.expense.id} className="flex items-center gap-3 py-2.5">
                    <div className="grid min-w-0 flex-1">
                      <span className="truncate text-sm font-medium">
                        {o.expense.description || o.expense.supplier}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {formatDate(o.expense.date)} · {costTypeLabel(o.expense.costType)}
                      </span>
                    </div>
                    <Money cents={o.result.reimbursable} strong />
                    {draft ? (
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={m.remove_from_claim()}
                        onClick={() => toggle("outlay", o.expense.id, false)}
                      >
                        <MinusIcon />
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>

          {p.advances.length ? (
            <Card>
              <CardHeader>
                <CardTitle>{m.advances()}</CardTitle>
              </CardHeader>
              <CardContent className="pt-2">
                <ul className="divide-y">
                  {p.advances.map((a) => (
                    <li key={a.id} className="flex items-center gap-3 py-2.5 text-sm">
                      <span className="flex-1">
                        {formatDate(a.date)}
                        {a.note ? ` · ${a.note}` : ""}
                      </span>
                      <Money cents={-a.amount} />
                      {draft ? (
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={m.remove_from_claim()}
                          onClick={() => toggle("advance", a.id, false)}
                        >
                          <MinusIcon />
                        </Button>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          ) : null}

          {draft &&
          (p.suggestions.trips.length || p.suggestions.outlays.length || p.suggestions.advances.length) ? (
            <Card className="border-dashed">
              <CardHeader>
                <CardTitle>{m.suggestions()}</CardTitle>
                <CardDescription>{m.suggestions_hint()}</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-1 pt-2">
                {p.suggestions.trips.map((t) => (
                  <SuggestionRow
                    key={`t${t.id}`}
                    label={t.purpose}
                    detail={formatRange(t.start, t.end)}
                    onAdd={() => toggle("trip", t.id, true)}
                  />
                ))}
                {p.suggestions.outlays.map((o) => (
                  <SuggestionRow
                    key={`o${o.id}`}
                    label={o.description || o.supplier || ""}
                    detail={formatDate(o.date)}
                    onAdd={() => toggle("outlay", o.id, true)}
                  />
                ))}
                {p.suggestions.advances.map((a) => (
                  <SuggestionRow
                    key={`a${a.id}`}
                    label={`${m.advance()} ${a.note ?? ""}`}
                    detail={formatDate(a.date)}
                    onAdd={() => toggle("advance", a.id, true)}
                  />
                ))}
              </CardContent>
            </Card>
          ) : null}

          {c.versions.length ? <VersionsCard p={p} /> : null}
        </div>

        <div className="grid h-fit gap-4 lg:sticky lg:top-6">
          <SummaryCard p={p} />
          <ActionsCard p={p} onDone={refresh} />
        </div>
      </div>
    </>
  );
}

function SuggestionRow({ label, detail, onAdd }: { label: string; detail: string; onAdd: () => void }) {
  return (
    <div className="flex items-center gap-3 py-1.5 text-sm">
      <span className="flex-1 truncate">
        {label} <span className="text-muted-foreground">· {detail}</span>
      </span>
      <Button variant="outline" size="sm" onClick={onAdd}>
        <PlusIcon />
        {m.add()}
      </Button>
    </div>
  );
}

function SummaryCard({ p }: { p: Preview }) {
  const t = p.totals;
  return (
    <Card>
      <CardHeader>
        <CardDescription>{t.repayment ? m.repayment() : m.payout()}</CardDescription>
        <p className={`text-3xl font-semibold tracking-tight ${t.repayment ? "text-destructive" : ""}`}>
          <Money cents={Math.abs(t.payout)} />
        </p>
      </CardHeader>
      <CardContent className="divide-y pt-2">
        <Row label={m.travel_reimbursement()}>
          <Money cents={t.travel} />
        </Row>
        <Row label={m.standalone_outlays()}>
          <Money cents={t.outlays} />
        </Row>
        <Row label={m.reimbursement_total()} strong>
          <Money cents={t.reimbursement} />
        </Row>
        <Row label={m.less_advances()}>
          <Money cents={-t.advances} />
        </Row>
      </CardContent>
    </Card>
  );
}

function IssuesCard({ p }: { p: Preview }) {
  if (!p.blockers.length && !p.warnings.length && !p.notices.length) {
    return (
      <Alert title={m.ready_to_submit()}>
        <span className="inline-flex items-center gap-1">
          <CheckCircle2Icon className="size-4 text-success" />
          {m.ready_to_submit_text()}
        </span>
      </Alert>
    );
  }
  return (
    <div className="grid gap-3">
      {p.blockers.length ? (
        <Alert variant="error" title={m.blockers_title()}>
          <IssueList issues={p.blockers} className="mt-1" />
        </Alert>
      ) : null}
      {p.warnings.length ? (
        <Alert variant="warning" title={m.warnings_title()}>
          <IssueList issues={p.warnings} className="mt-1" />
        </Alert>
      ) : null}
      {p.notices.length ? (
        <details className="rounded-lg border px-3 py-2 text-sm">
          <summary className="cursor-pointer text-muted-foreground">
            {m.notices_title({ count: String(p.notices.length) })}
          </summary>
          <IssueList issues={p.notices} className="mt-2" />
        </details>
      ) : null}
    </div>
  );
}

function ActionsCard({ p, onDone }: { p: Preview; onDone: () => void }) {
  const c = p.claim;
  const [ack, setAck] = useState(false);
  const [dialog, setDialog] = useState<"unlock" | "paid" | "unpaid" | null>(null);
  const [reason, setReason] = useState("");
  const [paidDate, setPaidDate] = useState(todayISO());
  const close = () => {
    setDialog(null);
    setReason("");
  };
  const submit = useMutation({
    ...submitClaimMutation(),
    onError: (e) => toast.error(errorText(e)),
    onSuccess: (r) => {
      onDone();
      toast.success(m.claim_submitted({ number: r.number }));
    },
  });
  const unlock = useMutation({
    ...unlockClaimMutation(),
    onSuccess: () => {
      onDone();
      close();
    },
  });
  const paid = useMutation({
    ...markClaimPaidMutation(),
    onSuccess: () => {
      onDone();
      close();
    },
  });
  const unpaid = useMutation({
    ...undoClaimPaidMutation(),
    onSuccess: () => {
      onDone();
      close();
    },
  });
  const id = c.id;
  const latest = c.versions[0];

  return (
    <Card>
      <CardContent className="grid gap-3">
        {c.status === "draft" ? (
          <>
            {p.warnings.length ? (
              <label className="flex items-start gap-2 text-sm">
                <Checkbox className="mt-0.5" checked={ack} onChange={(e) => setAck(e.target.checked)} />
                {m.acknowledge_warnings({ count: String(p.warnings.length) })}
              </label>
            ) : null}
            <Button
              size="lg"
              disabled={p.blockers.length > 0 || (p.warnings.length > 0 && !ack) || submit.isPending}
              onClick={() =>
                submit.mutate({
                  path: { id },
                  body: { revision: p.revision, acknowledgeWarnings: ack || p.warnings.length === 0 },
                })
              }
            >
              {submit.isPending ? <Loader2Icon className="animate-spin" /> : <SendIcon />}
              {c.currentVersion ? m.resubmit() : m.submit()}
            </Button>
            <p className="text-xs text-muted-foreground">{m.submit_hint()}</p>
          </>
        ) : null}
        {latest && c.status !== "draft" ? (
          <>
            <a
              href={`/api/claims/${id}/versions/${latest.version}/pdf`}
              target="_blank"
              rel="noreferrer"
              className={buttonVariants({ size: "lg" })}
            >
              <FileTextIcon />
              {m.open_pdf_export()}
            </a>
            <a
              href={`/api/claims/${id}/versions/${latest.version}/zip`}
              className={buttonVariants({ variant: "outline" })}
            >
              <FileArchiveIcon />
              {m.download_zip()}
            </a>
          </>
        ) : null}
        {c.status === "submitted" ? (
          <>
            <Button variant="secondary" onClick={() => setDialog("paid")}>
              <BanknoteIcon />
              {m.mark_paid()}
            </Button>
            <Button variant="ghost" onClick={() => setDialog("unlock")}>
              <LockOpenIcon />
              {m.unlock_for_correction()}
            </Button>
          </>
        ) : null}
        {c.status === "paid" ? (
          <>
            <Alert title={m.paid_on({ date: formatDate(c.paidAt) })}>{c.paidNote}</Alert>
            <Button variant="ghost" onClick={() => setDialog("unpaid")}>
              <Undo2Icon />
              {m.undo_paid()}
            </Button>
          </>
        ) : null}
      </CardContent>

      <Dialog open={dialog !== null} onOpenChange={(o) => !o && close()}>
        {dialog === "unlock" || dialog === "unpaid" ? (
          <DialogContent
            title={dialog === "unlock" ? m.unlock_for_correction() : m.undo_paid()}
            description={dialog === "unlock" ? m.unlock_hint() : undefined}
          >
            <form
              className="grid gap-3"
              onSubmit={(e) => {
                e.preventDefault();
                (dialog === "unlock" ? unlock : unpaid).mutate({ path: { id }, body: { reason } });
              }}
            >
              <Field label={m.reason()}>
                <Textarea value={reason} onChange={(e) => setReason(e.target.value)} required rows={3} />
              </Field>
              <Button type="submit">{m.confirm()}</Button>
            </form>
          </DialogContent>
        ) : null}
        {dialog === "paid" ? (
          <DialogContent title={m.mark_paid()} description={m.mark_paid_hint()}>
            <form
              className="grid gap-3"
              onSubmit={(e) => {
                e.preventDefault();
                paid.mutate({ path: { id }, body: { date: paidDate, note: reason || undefined } });
              }}
            >
              <Field label={m.payment_date()}>
                <Input type="date" value={paidDate} onChange={(e) => setPaidDate(e.target.value)} required />
              </Field>
              <Field label={m.note_optional()}>
                <Input value={reason} onChange={(e) => setReason(e.target.value)} />
              </Field>
              <Button type="submit">{m.mark_paid()}</Button>
            </form>
          </DialogContent>
        ) : null}
      </Dialog>
    </Card>
  );
}

function VersionsCard({ p }: { p: Preview }) {
  const c = p.claim;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.versions()}</CardTitle>
        <CardDescription>{m.versions_hint()}</CardDescription>
      </CardHeader>
      <CardContent className="pt-2">
        <ul className="divide-y">
          {c.versions.map((v) => (
            <li key={v.version} className="grid gap-1.5 py-3 sm:grid-cols-[1fr_auto] sm:items-center">
              <div className="grid gap-0.5">
                <span className="flex items-center gap-2 text-sm font-medium">
                  {m.version_n({ n: String(v.version) })}
                  {v.supersededAt ? (
                    <Badge variant="outline">{m.superseded()}</Badge>
                  ) : (
                    <Badge variant="success">{m.current()}</Badge>
                  )}
                </span>
                <span className="text-xs text-muted-foreground">
                  {m.submitted_on({ date: formatDate(v.submittedAt) })} · <Money cents={v.payout} />
                </span>
                {v.unlockReason ? (
                  <span className="text-xs text-muted-foreground">
                    {m.unlocked_because({ reason: v.unlockReason })}
                  </span>
                ) : null}
              </div>
              <div className="flex gap-2">
                <a
                  href={`/api/claims/${c.id}/versions/${v.version}/pdf`}
                  target="_blank"
                  rel="noreferrer"
                  className={buttonVariants({ variant: "outline", size: "sm" })}
                >
                  <DownloadIcon />
                  PDF
                </a>
                <a
                  href={`/api/claims/${c.id}/versions/${v.version}/zip`}
                  className={buttonVariants({ variant: "outline", size: "sm" })}
                >
                  <DownloadIcon />
                  ZIP
                </a>
              </div>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
