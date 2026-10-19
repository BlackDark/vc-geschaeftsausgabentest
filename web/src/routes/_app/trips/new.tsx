import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { ArrowLeftIcon } from "lucide-react";
import { toast } from "sonner";
import { createTripMutation } from "@/api/gen/@tanstack/react-query.gen";
import { PageHeader } from "@/components/common";
import { TripForm } from "@/components/trip-form";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/trips/new")({ component: NewTrip });

function NewTrip() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const create = useMutation({
    ...createTripMutation(),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: [{ _id: "listTrips" }] });
      toast.success(m.trip_saved());
      navigate({ to: "/trips/$tripId", params: { tripId: String(d.trip.id) } });
    },
  });
  return (
    <>
      <PageHeader
        title={m.new_trip()}
        back={
          <Link
            to="/trips"
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          >
            <ArrowLeftIcon className="size-4" />
            {m.nav_trips()}
          </Link>
        }
      />
      <TripForm
        submitLabel={m.save_trip()}
        pending={create.isPending}
        onSubmit={(body) => create.mutate({ body })}
      />
    </>
  );
}
