import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { ArrowLeftIcon } from "lucide-react";
import { toast } from "sonner";
import { getTripOptions, updateTripMutation } from "@/api/gen/@tanstack/react-query.gen";
import { PageHeader } from "@/components/common";
import { TripForm } from "@/components/trip-form";
import { Skeleton } from "@/components/ui/misc";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/trips/$tripId/edit")({ component: EditTrip });

function EditTrip() {
  const { tripId } = Route.useParams();
  const id = Number(tripId);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const { data } = useQuery(getTripOptions({ path: { id } }));
  const update = useMutation({
    ...updateTripMutation(),
    onSuccess: (d) => {
      qc.setQueryData(getTripOptions({ path: { id } }).queryKey, d);
      qc.invalidateQueries({ queryKey: [{ _id: "listTrips" }] });
      toast.success(m.trip_saved());
      navigate({ to: "/trips/$tripId", params: { tripId } });
    },
  });
  return (
    <>
      <PageHeader
        title={m.edit_trip()}
        back={
          <Link
            to="/trips/$tripId"
            params={{ tripId }}
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          >
            <ArrowLeftIcon className="size-4" />
            {data?.trip.purpose ?? m.nav_trips()}
          </Link>
        }
      />
      {data ? (
        <TripForm
          initial={data.trip}
          version={data.trip.version}
          submitLabel={m.save_changes()}
          pending={update.isPending}
          onSubmit={(body) => update.mutate({ path: { id }, body })}
        />
      ) : (
        <Skeleton className="h-96" />
      )}
    </>
  );
}
