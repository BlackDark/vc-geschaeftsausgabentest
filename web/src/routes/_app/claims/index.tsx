import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { ChevronRightIcon, FileTextIcon, PiggyBankIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
  createAdvanceMutation,
  createClaimMutation,
  deleteAdvanceMutation,
  listAdvancesOptions,
  listClaimsOptions,
} from "@/api/gen/@tanstack/react-query.gen";
import { ClaimStatusBadge, Money, PageHeader } from "@/components/common";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { EmptyState, Skeleton } from "@/components/ui/misc";
import { formatDate, monthOf, parseAmount, quarterOf, shiftPeriod, todayISO } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/claims/")({
  validateSearch: (s: Record<string, unknown>): { new?: boolean } => ({
    new: s.new === true || s.new === "true" ? true : undefined,
  }),
  component: Claims,
});

function Claims() {
  const search = Route.useSearch();
  const { data, isLoading } = useQuery(listClaimsOptions());
  const [creating, setCreating] = useState(!!search.new);
  const newButton = (
    <Button onClick={() => setCreating(true)}>
      <PlusIcon />
      {m.new_claim()}
    </Button>
  );
  return (
    <>
      <PageHeader title={m.nav_claims()} description={m.claims_subtitle()} actions={newButton} />
      {isLoading ? (
        <Skeleton className="h-40" />
      ) : !data?.length ? (
        <EmptyState
          icon={FileTextIcon}
          title={m.no_claims_yet()}
          text={m.no_claims_text()}
          action={newButton}
        />
      ) : (
        <Card className="divide-y overflow-hidden">
          {data.map((c) => (
            <Link
              key={c.id}
              to="/claims/$claimId"
              params={{ claimId: String(c.id) }}
              className="flex items-center gap-3 px-4 py-3.5 hover:bg-accent/60 sm:px-5"
            >
              <div className="grid min-w-0 flex-1 gap-1">
                <span className="truncate font-medium">{c.title || m.claim_untitled()}</span>
                <span className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  {formatDate(c.periodStart)} – {formatDate(c.periodEnd)}
                  {c.number ? <span>· {c.number}</span> : null}
                  {c.currentVersion > 1 ? <Badge variant="outline">v{c.currentVersion}</Badge> : null}
                </span>
              </div>
              {c.status !== "draft" ? <Money cents={c.payout} strong /> : null}
              <ClaimStatusBadge status={c.status} />
              <ChevronRightIcon className="size-4 text-muted-foreground" />
            </Link>
          ))}
        </Card>
      )}
      <Advances />
      <NewClaimDialog open={creating} onOpenChange={setCreating} />
    </>
  );
}

function NewClaimDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const today = todayISO();
  const presets = [
    { key: "q", label: m.this_quarter(), ...quarterOf(today) },
    { key: "lq", label: m.last_quarter(), ...shiftPeriod(quarterOf(today), -3) },
    { key: "m", label: m.this_month(), ...monthOf(today) },
    { key: "lm", label: m.last_month(), ...shiftPeriod(monthOf(today), -1) },
  ];
  const [period, setPeriod] = useState({ start: presets[0]!.start, end: presets[0]!.end });
  const [title, setTitle] = useState("");
  const create = useMutation({
    ...createClaimMutation(),
    onSuccess: (c) => {
      qc.invalidateQueries({ queryKey: [{ _id: "listClaims" }] });
      onOpenChange(false);
      navigate({ to: "/claims/$claimId", params: { claimId: String(c.id) } });
    },
  });
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent title={m.new_claim()} description={m.new_claim_hint()}>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate({
              body: { title: title || undefined, periodStart: period.start, periodEnd: period.end },
            });
          }}
        >
          <div className="grid grid-cols-2 gap-2">
            {presets.map((p) => (
              <button
                key={p.key}
                type="button"
                onClick={() => setPeriod({ start: p.start, end: p.end })}
                className={cn(
                  "rounded-lg border px-3 py-2 text-left text-sm",
                  period.start === p.start && period.end === p.end
                    ? "border-primary bg-accent"
                    : "hover:bg-accent",
                )}
              >
                <span className="block font-medium">{p.label}</span>
                <span className="text-xs text-muted-foreground">
                  {formatDate(p.start)} – {formatDate(p.end)}
                </span>
              </button>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Field label={m.from()}>
              <Input
                type="date"
                value={period.start}
                onChange={(e) => setPeriod({ ...period, start: e.target.value })}
                required
              />
            </Field>
            <Field label={m.to()}>
              <Input
                type="date"
                value={period.end}
                min={period.start}
                onChange={(e) => setPeriod({ ...period, end: e.target.value })}
                required
              />
            </Field>
          </div>
          <Field label={m.claim_title_optional()}>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={m.claim_title_placeholder()}
            />
          </Field>
          <Button type="submit" disabled={create.isPending}>
            {m.create_claim()}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function Advances() {
  const qc = useQueryClient();
  const { data = [] } = useQuery(listAdvancesOptions());
  const [open, setOpen] = useState(false);
  const [f, setF] = useState({ date: todayISO(), amount: "", note: "" });
  const refresh = () => qc.invalidateQueries({ queryKey: [{ _id: "listAdvances" }] });
  const create = useMutation({
    ...createAdvanceMutation(),
    onSuccess: () => {
      refresh();
      setOpen(false);
      toast.success(m.advance_saved());
    },
  });
  const del = useMutation({ ...deleteAdvanceMutation(), onSuccess: refresh });
  return (
    <Card className="mt-6">
      <CardHeader className="flex-row items-start justify-between gap-3">
        <div className="grid gap-1">
          <CardTitle>{m.advances()}</CardTitle>
          <CardDescription>{m.advances_hint()}</CardDescription>
        </div>
        <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
          <PiggyBankIcon />
          {m.add_advance()}
        </Button>
      </CardHeader>
      <CardContent className="pt-2">
        {data.length === 0 ? (
          <p className="text-sm text-muted-foreground">{m.no_advances()}</p>
        ) : (
          <ul className="divide-y">
            {data.map((a) => (
              <li key={a.id} className="flex items-center justify-between gap-3 py-2 text-sm">
                <span className="grid">
                  <span>
                    {formatDate(a.date)}
                    {a.note ? ` · ${a.note}` : ""}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {a.claimId ? m.in_claim() : m.not_claimed()}
                  </span>
                </span>
                <span className="flex items-center gap-2">
                  <Money cents={a.amount} />
                  {!a.claimId ? (
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={m.delete()}
                      onClick={() => del.mutate({ path: { id: a.id } })}
                    >
                      <Trash2Icon />
                    </Button>
                  ) : null}
                </span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent title={m.add_advance()} description={m.advances_hint()}>
          <form
            className="grid gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              create.mutate({
                body: { date: f.date, amount: parseAmount(f.amount) ?? 0, note: f.note || undefined },
              });
            }}
          >
            <div className="grid grid-cols-2 gap-3">
              <Field label={m.date()}>
                <Input
                  type="date"
                  value={f.date}
                  onChange={(e) => setF({ ...f, date: e.target.value })}
                  required
                />
              </Field>
              <Field label={m.amount()}>
                <Input
                  inputMode="decimal"
                  value={f.amount}
                  onChange={(e) => setF({ ...f, amount: e.target.value })}
                  required
                />
              </Field>
            </div>
            <Field label={m.note_optional()}>
              <Input value={f.note} onChange={(e) => setF({ ...f, note: e.target.value })} />
            </Field>
            <Button type="submit" disabled={create.isPending}>
              {m.save()}
            </Button>
          </form>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
