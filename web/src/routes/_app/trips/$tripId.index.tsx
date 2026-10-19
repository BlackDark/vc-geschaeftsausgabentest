import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import {
  ArrowLeftIcon,
  CarIcon,
  LockIcon,
  MoreHorizontalIcon,
  PencilIcon,
  PlusIcon,
  ReceiptIcon,
  Trash2Icon,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
  createExpenseMutation,
  deleteExpenseMutation,
  deleteTripMutation,
  getTripOptions,
  updateExpenseMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import type { Day, Expense, ExpenseResult, TripDetail } from "@/api/gen/types.gen";
import { costTypeLabel, IssueList, Money, PageHeader, Row } from "@/components/common";
import { ExpenseForm } from "@/components/expense-form";
import { nightKinds } from "@/components/trip-form";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Menu, MenuContent, MenuItem, MenuTrigger } from "@/components/ui/menu";
import { Alert, Skeleton } from "@/components/ui/misc";
import { datesBetween, formatCents, formatLocalDateTime, formatMinor, formatWeekday } from "@/lib/format";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/trips/$tripId/")({ component: TripPage });

const dayTypeLabel = (t: string) =>
  ({
    full: m.day_full(),
    partial: m.day_partial(),
    one_day: m.day_one_day(),
    none: m.day_none(),
    assigned_elsewhere: m.day_assigned(),
  })[t] ?? t;

function TripPage() {
  const { tripId } = Route.useParams();
  const id = Number(tripId);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const query = getTripOptions({ path: { id } });
  const { data, isLoading, error } = useQuery(query);
  const del = useMutation({
    ...deleteTripMutation(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [{ _id: "listTrips" }] });
      toast.success(m.trip_deleted());
      navigate({ to: "/trips" });
    },
  });
  if (isLoading) return <Skeleton className="h-96" />;
  if (error || !data) return <Alert variant="error">{m.not_found()}</Alert>;
  const { trip, result } = data;
  const t = result.totals;
  const back = (
    <Link
      to="/trips"
      className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
    >
      <ArrowLeftIcon className="size-4" />
      {m.nav_trips()}
    </Link>
  );
  return (
    <>
      <PageHeader
        back={back}
        title={trip.purpose}
        description={`${formatLocalDateTime(trip.start)} – ${formatLocalDateTime(trip.end)}${trip.destination ? ` · ${trip.destination}` : ""}`}
        actions={
          trip.locked ? null : (
            <>
              <Link
                to="/trips/$tripId/edit"
                params={{ tripId }}
                className={buttonVariants({ variant: "outline" })}
              >
                <PencilIcon />
                {m.edit()}
              </Link>
              <Menu>
                <MenuTrigger
                  className={buttonVariants({ variant: "ghost", size: "icon" })}
                  aria-label={m.more_actions()}
                >
                  <MoreHorizontalIcon />
                </MenuTrigger>
                <MenuContent>
                  <MenuItem
                    className="text-destructive"
                    onClick={() => {
                      if (confirm(m.confirm_delete_trip())) del.mutate({ path: { id } });
                    }}
                  >
                    <Trash2Icon />
                    {m.delete()}
                  </MenuItem>
                </MenuContent>
              </Menu>
            </>
          )
        }
      />

      {trip.locked ? (
        <Alert className="mb-4" title={m.trip_locked({ claim: trip.claimLabel ?? "" })}>
          <span className="inline-flex items-center gap-1">
            <LockIcon className="size-3.5" />
            {m.trip_locked_text()}
          </span>
        </Alert>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-[1fr_20rem]">
        <div className="grid min-w-0 gap-4">
          {data.issues.length ? (
            <Card>
              <CardContent>
                <IssueList issues={data.issues} />
              </CardContent>
            </Card>
          ) : null}
          <ExpensesCard data={data} />
          <MealsCard days={result.days} total={t.meals} />
          {result.nights.length ? (
            <Card>
              <CardHeader>
                <CardTitle>{m.lodging_title()}</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-1 pt-2">
                {result.nights.map((n) => (
                  <div
                    key={n.date}
                    className="flex items-center justify-between gap-3 border-b py-2 text-sm last:border-0"
                  >
                    <div className="grid">
                      <span className="font-medium">{m.night_of({ date: formatWeekday(n.date) })}</span>
                      <span className="text-xs text-muted-foreground">
                        {nightKinds().find((k) => k.value === n.kind)?.label ?? m.not_set()}
                      </span>
                    </div>
                    {n.kind === "private_allowance" ? (
                      <Money cents={n.allowance} />
                    ) : (
                      <span className="text-xs text-muted-foreground">
                        {n.kind === "private_actual" ? m.via_invoice() : "–"}
                      </span>
                    )}
                  </div>
                ))}
              </CardContent>
            </Card>
          ) : null}
          {result.journeys.length ? (
            <Card>
              <CardHeader>
                <CardTitle>{m.mileage_title()}</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-1 pt-2">
                {result.journeys.map((j) => (
                  <div
                    key={j.journeyId}
                    className="flex items-center justify-between gap-3 border-b py-2 text-sm last:border-0"
                  >
                    <span className="flex items-center gap-2">
                      <CarIcon className="size-4 text-muted-foreground" />
                      {formatWeekday(j.date)} · {(j.distanceMeters / 1000).toLocaleString()} km ×{" "}
                      {formatCents(j.centsPerKm)}
                    </span>
                    <Money cents={j.amount} />
                  </div>
                ))}
              </CardContent>
            </Card>
          ) : null}
        </div>

        <Card className="h-fit lg:sticky lg:top-6">
          <CardHeader>
            <CardDescription>{m.trip_total()}</CardDescription>
            <p className="text-3xl font-semibold tracking-tight">
              <Money cents={t.total} />
            </p>
          </CardHeader>
          <CardContent className="divide-y pt-2">
            <Row label={m.meals_allowance()}>
              <Money cents={t.meals} />
            </Row>
            <Row label={m.lodging_allowance()}>
              <Money cents={t.lodgingAllowance} />
            </Row>
            <Row label={m.mileage()}>
              <Money cents={t.mileage} />
            </Row>
            <Row label={m.expenses_reimbursed()}>
              <Money cents={t.expenses} />
            </Row>
            {t.companyPaid > 0 ? (
              <Row label={m.company_paid_info()}>
                <span className="text-muted-foreground">
                  <Money cents={t.companyPaid} />
                </span>
              </Row>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </>
  );
}

function MealsCard({ days, total }: { days: Day[]; total: number }) {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <div className="grid gap-1">
          <CardTitle>{m.meals_allowance()}</CardTitle>
          <CardDescription>{m.meals_allowance_hint()}</CardDescription>
        </div>
        <Money cents={total} strong />
      </CardHeader>
      <CardContent className="grid gap-0 pt-2">
        {days.map((d) => (
          <div key={d.date} className="grid gap-1 border-b py-2.5 text-sm last:border-0">
            <div className="flex items-center justify-between gap-3">
              <div className="grid">
                <span className="font-medium">{formatWeekday(d.date)}</span>
                <span className="text-xs text-muted-foreground">
                  {dayTypeLabel(d.type)} · {d.location.city ? `${d.location.city}, ` : ""}
                  {d.location.country}
                  {d.type === "one_day" || d.type === "none"
                    ? ` · ${Math.floor(d.absenceMinutes / 60)}:${String(d.absenceMinutes % 60).padStart(2, "0")} h`
                    : ""}
                </span>
              </div>
              <Money cents={d.payable} strong />
            </div>
            {d.base > 0 || d.deductions.length ? (
              <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
                <span>
                  {m.allowance()} {formatCents(d.base)}
                </span>
                {d.deductions.map((x) => (
                  <span key={x.meal}>
                    −{" "}
                    {
                      (
                        { breakfast: m.breakfast(), lunch: m.lunch(), dinner: m.dinner() } as Record<
                          string,
                          string
                        >
                      )[x.meal]
                    }{" "}
                    {formatCents(x.amount)}
                    {x.contribution > 0 ? ` (${m.contribution()} ${formatCents(x.contribution)})` : ""}
                  </span>
                ))}
                {d.excluded ? <span>{m.excluded()}</span> : null}
                {d.allocatedTo ? <span>{m.allocated_to_other()}</span> : null}
                {d.frozenElsewhere > 0 ? (
                  <span>{m.frozen_elsewhere({ amount: formatCents(d.frozenElsewhere) })}</span>
                ) : null}
                <span className="opacity-70">[{d.ruleIds.join(", ")}]</span>
              </div>
            ) : null}
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

function ExpensesCard({ data }: { data: TripDetail }) {
  const { trip, result } = data;
  const qc = useQueryClient();
  const [editing, setEditing] = useState<Expense | "new" | null>(null);
  const nights = datesBetween(trip.startDate, trip.endDate).slice(0, -1);
  const refresh = () => {
    qc.invalidateQueries({ queryKey: getTripOptions({ path: { id: trip.id } }).queryKey });
    qc.invalidateQueries({ queryKey: [{ _id: "listTrips" }] });
    qc.invalidateQueries({ queryKey: [{ _id: "listReceipts" }] });
  };
  const create = useMutation({
    ...createExpenseMutation(),
    onSuccess: () => {
      refresh();
      setEditing(null);
      toast.success(m.expense_saved());
    },
  });
  const update = useMutation({
    ...updateExpenseMutation(),
    onSuccess: () => {
      refresh();
      setEditing(null);
      toast.success(m.expense_saved());
    },
  });
  const del = useMutation({
    ...deleteExpenseMutation(),
    onSuccess: () => {
      refresh();
      setEditing(null);
    },
  });
  const results = new Map<number, ExpenseResult>(result.expenses.map((r) => [r.expenseId, r]));
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <CardTitle>{m.expenses()}</CardTitle>
        {!trip.locked ? (
          <Button size="sm" onClick={() => setEditing("new")}>
            <PlusIcon />
            {m.add_expense()}
          </Button>
        ) : null}
      </CardHeader>
      <CardContent className="pt-2">
        {data.expenses.length === 0 ? (
          <p className="text-sm text-muted-foreground">{m.no_expenses()}</p>
        ) : (
          <ul className="divide-y">
            {data.expenses.map((e) => {
              const r = results.get(e.id);
              return (
                <li key={e.id}>
                  <button
                    type="button"
                    disabled={trip.locked}
                    onClick={() => setEditing(e)}
                    className="-mx-2 flex w-[calc(100%+1rem)] items-center gap-3 rounded-md px-2 py-2.5 text-left hover:bg-accent disabled:hover:bg-transparent"
                  >
                    <div className="grid min-w-0 flex-1 gap-0.5">
                      <span className="truncate text-sm font-medium">
                        {e.supplier || costTypeLabel(e.costType)}
                      </span>
                      <span className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                        {formatWeekday(e.date)} · {costTypeLabel(e.costType)}
                        {e.payer === "employer" ? (
                          <Badge variant="outline">{m.payer_employer()}</Badge>
                        ) : null}
                        {e.receipts.length ? (
                          e.receipts.map((rc) => (
                            <Badge key={rc.id} variant={rc.status === "confirmed" ? "default" : "warning"}>
                              <ReceiptIcon className="size-3" />
                              {rc.number || m.receipt_unconfirmed()}
                            </Badge>
                          ))
                        ) : (
                          <Badge variant="warning">{m.receipt_missing()}</Badge>
                        )}
                      </span>
                    </div>
                    <div className="grid text-right">
                      <Money cents={r?.reimbursable} strong />
                      {r && r.reimbursable !== r.grossEur ? (
                        <span className="text-xs text-muted-foreground line-through">
                          {formatCents(r.grossEur)}
                        </span>
                      ) : e.currency !== "EUR" ? (
                        <span className="text-xs text-muted-foreground">
                          {formatMinor(e.grossMinor, e.currency)}
                        </span>
                      ) : null}
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </CardContent>
      <Dialog open={editing !== null} onOpenChange={(o) => !o && setEditing(null)}>
        {editing !== null ? (
          <DialogContent title={editing === "new" ? m.add_expense() : m.edit_expense()} wide>
            <ExpenseForm
              tripId={trip.id}
              nights={nights}
              tripRange={{ start: trip.startDate, end: trip.endDate }}
              expense={editing === "new" ? undefined : editing}
              pending={create.isPending || update.isPending}
              onSubmit={(body) =>
                editing === "new"
                  ? create.mutate({ body })
                  : update.mutate({ path: { id: editing.id }, body })
              }
            />
            {editing !== "new" ? (
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
    </Card>
  );
}
