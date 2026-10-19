import { AlertTriangleIcon, InfoIcon, OctagonXIcon } from "lucide-react";
import type * as React from "react";
import type { Issue } from "@/api/gen/types.gen";
import { Badge } from "@/components/ui/badge";
import { issueText } from "@/lib/api";
import { formatCents, formatDate } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

export function PageHeader({
  title,
  description,
  actions,
  back,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  back?: React.ReactNode;
}) {
  return (
    <div className="mb-6 grid gap-3">
      {back}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="grid min-w-0 gap-1">
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          {description ? <p className="text-sm text-muted-foreground">{description}</p> : null}
        </div>
        {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
      </div>
    </div>
  );
}

export function Money({
  cents,
  className,
  strong,
}: {
  cents: number | undefined;
  className?: string;
  strong?: boolean;
}) {
  return (
    <span className={cn("tabular whitespace-nowrap", strong && "font-semibold", className)}>
      {formatCents(cents)}
    </span>
  );
}

export function ClaimStatusBadge({ status }: { status: string }) {
  switch (status) {
    case "paid":
      return <Badge variant="success">{m.status_paid()}</Badge>;
    case "submitted":
      return <Badge variant="primary">{m.status_submitted()}</Badge>;
    default:
      return <Badge variant="outline">{m.status_draft()}</Badge>;
  }
}

const issueIcon = { blocker: OctagonXIcon, warning: AlertTriangleIcon, notice: InfoIcon } as const;
const issueColor = {
  blocker: "text-destructive",
  warning: "text-warning",
  notice: "text-muted-foreground",
} as const;

/** Lists blockers, warnings and notices with translated texts. */
export function IssueList({ issues, className }: { issues: Issue[]; className?: string }) {
  if (!issues.length) return null;
  return (
    <ul className={cn("grid gap-2", className)}>
      {issues.map((is, i) => {
        const sev = (is.severity as keyof typeof issueIcon) ?? "notice";
        const Icon = issueIcon[sev] ?? InfoIcon;
        return (
          <li key={`${is.code}-${i}`} className="flex gap-2 text-sm">
            <Icon className={cn("mt-0.5 size-4 shrink-0", issueColor[sev])} />
            <span>
              {issueText(is.code, is.params)}
              {is.date ? <span className="text-muted-foreground"> · {formatDate(is.date)}</span> : null}
              <span className="ml-1 text-xs text-muted-foreground">[{is.ruleId}]</span>
            </span>
          </li>
        );
      })}
    </ul>
  );
}

export function IssueCounts({ blockers, warnings }: { blockers: number; warnings: number }) {
  return (
    <>
      {blockers > 0 ? (
        <Badge variant="destructive">
          <OctagonXIcon className="size-3" />
          {m.n_blockers({ count: String(blockers) })}
        </Badge>
      ) : null}
      {warnings > 0 ? (
        <Badge variant="warning">
          <AlertTriangleIcon className="size-3 text-warning" />
          {m.n_warnings({ count: String(warnings) })}
        </Badge>
      ) : null}
    </>
  );
}

/** Key/value rows for summaries. */
export function Row({
  label,
  children,
  strong,
}: {
  label: React.ReactNode;
  children: React.ReactNode;
  strong?: boolean;
}) {
  return (
    <div
      className={cn("flex items-baseline justify-between gap-4 py-1.5 text-sm", strong && "font-semibold")}
    >
      <span className={cn(!strong && "text-muted-foreground")}>{label}</span>
      <span className="text-right">{children}</span>
    </div>
  );
}

export const costTypeLabel = (t: string): string =>
  ({
    transport: m.cost_transport(),
    meals: m.cost_meals(),
    lodging: m.cost_lodging(),
    incidental: m.cost_incidental(),
    hospitality: m.cost_hospitality(),
    tools_equipment: m.cost_tools_equipment(),
    software_services: m.cost_software_services(),
    other_outlay: m.cost_other_outlay(),
  })[t] ?? t;
