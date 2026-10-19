import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";
import { ChevronRightIcon, LockIcon, MapIcon, PlusIcon } from "lucide-react";
import { listTripsOptions } from "@/api/gen/@tanstack/react-query.gen";
import { IssueCounts, Money, PageHeader } from "@/components/common";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState, Skeleton } from "@/components/ui/misc";
import { formatRange } from "@/lib/format";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/trips/")({ component: Trips });

function Trips() {
  const { data, isLoading } = useQuery(listTripsOptions());
  const newButton = (
    <Link to="/trips/new" className={buttonVariants()}>
      <PlusIcon />
      {m.new_trip()}
    </Link>
  );
  return (
    <>
      <PageHeader title={m.nav_trips()} description={m.trips_subtitle()} actions={newButton} />
      {isLoading ? (
        <Skeleton className="h-40" />
      ) : !data?.length ? (
        <EmptyState icon={MapIcon} title={m.no_trips()} text={m.no_trips_text()} action={newButton} />
      ) : (
        <Card className="divide-y overflow-hidden">
          {data.map((t) => (
            <Link
              key={t.id}
              to="/trips/$tripId"
              params={{ tripId: String(t.id) }}
              className="flex items-center gap-3 px-4 py-3.5 hover:bg-accent/60 sm:px-5"
            >
              <div className="grid min-w-0 flex-1 gap-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="truncate font-medium">{t.purpose}</span>
                  {t.destination ? (
                    <span className="truncate text-sm text-muted-foreground">· {t.destination}</span>
                  ) : null}
                </div>
                <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>{formatRange(t.start, t.end)}</span>
                  {t.claimLabel ? (
                    <Badge variant="outline">
                      {t.locked ? <LockIcon className="size-3" /> : null}
                      {t.claimLabel}
                    </Badge>
                  ) : (
                    <Badge variant="default">{m.not_claimed()}</Badge>
                  )}
                  <IssueCounts blockers={t.blockers} warnings={t.warnings} />
                </div>
              </div>
              <Money cents={t.total} strong />
              <ChevronRightIcon className="size-4 text-muted-foreground" />
            </Link>
          ))}
        </Card>
      )}
    </>
  );
}
