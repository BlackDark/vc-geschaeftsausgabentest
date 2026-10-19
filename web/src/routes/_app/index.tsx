import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import {
  ArrowRightIcon,
  BuildingIcon,
  FileTextIcon,
  InboxIcon,
  MapIcon,
  PlusIcon,
  WalletIcon,
} from "lucide-react";
import { getDashboardOptions } from "@/api/gen/@tanstack/react-query.gen";
import { ClaimStatusBadge, Money, PageHeader } from "@/components/common";
import { ReceiptCapture } from "@/components/receipt-capture";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, Skeleton } from "@/components/ui/misc";
import { formatDate } from "@/lib/format";
import { useUser } from "@/lib/session";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/")({ component: Dashboard });

function Dashboard() {
  const user = useUser();
  const { data, isLoading } = useQuery(getDashboardOptions());
  const firstName = user.name.split(" ")[0];
  return (
    <>
      <PageHeader title={m.hello({ name: firstName ?? user.name })} description={m.dashboard_subtitle()} />
      {data && !data.hasEmployer ? (
        <Alert variant="warning" title={m.setup_employer_title()} className="mb-6">
          <p>{m.setup_employer_text()}</p>
          <Link to="/settings" className={buttonVariants({ size: "sm", className: "mt-2" })}>
            <BuildingIcon />
            {m.setup_employer_action()}
          </Link>
        </Alert>
      ) : null}

      <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <QuickAction to="/trips/new" icon={MapIcon} label={m.new_trip()} />
        <QuickAction to="/outlays" search={{ new: true }} icon={WalletIcon} label={m.new_outlay()} />
        <QuickAction to="/claims" search={{ new: true }} icon={FileTextIcon} label={m.new_claim()} />
        <div className="hidden sm:block">
          <ReceiptCapture className="h-full min-h-20 w-full flex-col gap-1 rounded-xl" variant="outline" />
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <StatCard
          title={m.open_amount()}
          loading={isLoading}
          value={<Money cents={data?.unclaimedTotal} />}
          text={m.open_amount_text({
            trips: String(data?.unclaimedTrips ?? 0),
            outlays: String(data?.unclaimedOutlays ?? 0),
          })}
          link={{ to: "/claims", label: m.create_claim() }}
        />
        <StatCard
          title={m.receipt_inbox()}
          loading={isLoading}
          value={<span className="tabular">{data?.inboxReceipts ?? 0}</span>}
          text={m.inbox_text()}
          link={{ to: "/receipts", label: m.open_inbox() }}
          icon={InboxIcon}
        />
        <StatCard
          title={m.awaiting_payment()}
          loading={isLoading}
          value={<Money cents={data?.openPayout} />}
          text={m.awaiting_payment_text()}
          link={{ to: "/claims", label: m.nav_claims() }}
        />
      </div>

      <Card className="mt-6">
        <CardHeader className="flex-row items-center justify-between">
          <CardTitle>{m.recent_claims()}</CardTitle>
          <Link to="/claims" className={buttonVariants({ variant: "ghost", size: "sm" })}>
            {m.show_all()}
            <ArrowRightIcon />
          </Link>
        </CardHeader>
        <CardContent className="pt-2">
          {isLoading ? (
            <Skeleton className="h-16" />
          ) : data?.claims.length ? (
            <ul className="divide-y">
              {data.claims.map((c) => (
                <li key={c.id}>
                  <Link
                    to="/claims/$claimId"
                    params={{ claimId: String(c.id) }}
                    className="-mx-2 flex items-center justify-between gap-3 rounded-md px-2 py-3 hover:bg-accent"
                  >
                    <div className="grid min-w-0">
                      <span className="truncate font-medium">
                        {c.title || c.number || m.claim_untitled()}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {formatDate(c.periodStart)} – {formatDate(c.periodEnd)}
                        {c.number ? ` · ${c.number}` : ""}
                      </span>
                    </div>
                    <div className="flex shrink-0 items-center gap-3">
                      {c.status !== "draft" ? <Money cents={c.payout} strong /> : null}
                      <ClaimStatusBadge status={c.status} />
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          ) : (
            <p className="py-4 text-sm text-muted-foreground">{m.no_claims_yet()}</p>
          )}
        </CardContent>
      </Card>
    </>
  );
}

function QuickAction({
  to,
  search,
  icon: Icon,
  label,
}: {
  to: string;
  search?: Record<string, unknown>;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
}) {
  return (
    <Link
      to={to}
      search={search as never}
      className="flex min-h-20 flex-col items-center justify-center gap-1.5 rounded-xl border bg-card p-3 text-center text-sm font-medium shadow-xs hover:bg-accent"
    >
      <span className="rounded-full bg-accent p-2 text-accent-foreground">
        <Icon className="size-4.5" />
      </span>
      <span className="flex items-center gap-1">
        <PlusIcon className="size-3.5" />
        {label}
      </span>
    </Link>
  );
}

function StatCard({
  title,
  value,
  text,
  link,
  loading,
  icon: Icon,
}: {
  title: string;
  value: React.ReactNode;
  text: string;
  link: { to: string; label: string };
  loading?: boolean;
  icon?: React.ComponentType<{ className?: string }>;
}) {
  return (
    <Card>
      <CardContent className="grid gap-2">
        <p className="flex items-center gap-2 text-sm text-muted-foreground">
          {Icon ? <Icon className="size-4" /> : null}
          {title}
        </p>
        {loading ? (
          <Skeleton className="h-8 w-28" />
        ) : (
          <p className="text-2xl font-semibold tracking-tight">{value}</p>
        )}
        <p className="text-xs text-muted-foreground">{text}</p>
        <Link
          to={link.to}
          className="mt-1 inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
        >
          {link.label}
          <ArrowRightIcon className="size-3.5" />
        </Link>
      </CardContent>
    </Card>
  );
}
