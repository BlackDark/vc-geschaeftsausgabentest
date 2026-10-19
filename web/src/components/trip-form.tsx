import { useQuery } from "@tanstack/react-query";
import { ChevronDownIcon, InfoIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useMemo, useState } from "react";
import { listLocationsOptions } from "@/api/gen/@tanstack/react-query.gen";
import type { DayDto, JourneyDto, NightDto, StopDto, TripInput } from "@/api/gen/types.gen";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox, Field, Input, Label, Select } from "@/components/ui/input";
import { datesBetween, formatWeekday, minorToInput, parseAmount } from "@/lib/format";
import { m } from "@/paraglide/messages";

const defaultTZ = "Europe/Berlin";

export const nightKinds = () =>
  [
    { value: "private_actual", label: m.night_private_actual(), hint: m.night_private_actual_hint() },
    {
      value: "private_allowance",
      label: m.night_private_allowance(),
      hint: m.night_private_allowance_hint(),
    },
    { value: "employer", label: m.night_employer(), hint: m.night_employer_hint() },
    { value: "in_transit", label: m.night_in_transit(), hint: m.night_in_transit_hint() },
    { value: "none", label: m.night_none(), hint: m.night_none_hint() },
    { value: "private_none", label: m.night_private_none(), hint: m.night_private_none_hint() },
  ] as const;

type FormState = {
  purpose: string;
  destination: string;
  project: string;
  notes: string;
  startDate: string;
  startTime: string;
  endDate: string;
  endTime: string;
  tz: string;
  foreign: boolean;
  stops: StopDto[];
  days: Record<string, DayDto>;
  nights: Record<string, string>;
  journeys: (JourneyDto & { km: string; roundTrip: boolean })[];
};

function toState(t?: TripInput): FormState {
  const days: Record<string, DayDto> = {};
  for (const d of t?.days ?? []) days[d.date] = d;
  const nights: Record<string, string> = {};
  for (const n of t?.nights ?? []) nights[n.date] = n.kind;
  return {
    purpose: t?.purpose ?? "",
    destination: t?.destination ?? "",
    project: t?.project ?? "",
    notes: t?.notes ?? "",
    startDate: t?.start.slice(0, 10) ?? "",
    startTime: t?.start.slice(11, 16) ?? "08:00",
    endDate: t?.end.slice(0, 10) ?? "",
    endTime: t?.end.slice(11, 16) ?? "18:00",
    tz: t?.startTz ?? defaultTZ,
    foreign: (t?.stops?.length ?? 0) > 0,
    stops: t?.stops ?? [],
    days,
    nights,
    journeys: (t?.journeys ?? []).map((j) => ({
      ...j,
      km: minorToInput(j.distanceMeters, 3).replace(/[.,]?0+$/, ""),
      roundTrip: false,
    })),
  };
}

/** Builds the API input; only days with entries are sent. */
function toInput(s: FormState, version?: number): TripInput {
  const range = s.startDate && s.endDate ? datesBetween(s.startDate, s.endDate) : [];
  const days = range
    .map((d) => s.days[d])
    .filter(
      (d): d is DayDto =>
        !!d &&
        (d.breakfast || d.lunch || d.dinner || d.excluded || !!d.overrideCountry || !!d.breakfastOrigin),
    );
  const nights: NightDto[] = range
    .slice(0, -1)
    .filter((d) => s.nights[d])
    .map((d) => ({ date: d, kind: s.nights[d] as NightDto["kind"] }));
  return {
    purpose: s.purpose,
    destination: s.destination || undefined,
    project: s.project || undefined,
    notes: s.notes || undefined,
    start: `${s.startDate}T${s.startTime}`,
    startTz: s.tz,
    end: `${s.endDate}T${s.endTime}`,
    endTz: s.tz,
    stops: s.foreign ? s.stops : [],
    days,
    nights,
    journeys: s.journeys
      .filter((j) => j.km)
      .map((j) => {
        const meters = (parseAmount(j.km, 3) ?? 0) * (j.roundTrip ? 2 : 1);
        return {
          date: j.date || s.startDate,
          vehicle: j.vehicle,
          distanceMeters: meters,
          description: j.description || undefined,
        };
      }),
    version,
  };
}

export function TripForm({
  initial,
  version,
  pending,
  onSubmit,
  submitLabel,
}: {
  initial?: TripInput;
  version?: number;
  pending?: boolean;
  onSubmit: (input: TripInput) => void;
  submitLabel: string;
}) {
  const [s, setS] = useState<FormState>(() => toState(initial));
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => setS((prev) => ({ ...prev, [k]: v }));
  const range = useMemo(
    () => (s.startDate && s.endDate && s.endDate >= s.startDate ? datesBetween(s.startDate, s.endDate) : []),
    [s.startDate, s.endDate],
  );
  const nightDates = range.slice(0, -1);
  const year = Number((s.startDate || new Date().toISOString()).slice(0, 4));
  const { data: locations = [] } = useQuery({
    ...listLocationsOptions({ query: { year } }),
    enabled: s.foreign,
  });
  const countries = useMemo(() => {
    const map = new Map<string, string>();
    for (const l of locations) if (l.country !== "DE") map.set(l.country, l.label);
    return [...map.entries()].sort((a, b) => a[1].localeCompare(b[1]));
  }, [locations]);

  const day = (d: string): DayDto => s.days[d] ?? { date: d };
  const setDay = (d: string, patch: Partial<DayDto>) =>
    set("days", { ...s.days, [d]: { ...day(d), ...patch } });

  const tooLong = range.length > 62;
  return (
    <form
      className="grid gap-5"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(toInput(s, version));
      }}
    >
      <Card>
        <CardHeader>
          <CardTitle>{m.trip_basics()}</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <Field label={m.trip_purpose()} htmlFor="purpose" className="sm:col-span-2">
            <Input
              id="purpose"
              value={s.purpose}
              onChange={(e) => set("purpose", e.target.value)}
              required
              placeholder={m.trip_purpose_placeholder()}
            />
          </Field>
          <Field label={m.trip_destination()} htmlFor="destination">
            <Input
              id="destination"
              value={s.destination}
              onChange={(e) => set("destination", e.target.value)}
              placeholder={m.trip_destination_placeholder()}
            />
          </Field>
          <Field label={m.trip_project()} htmlFor="project">
            <Input id="project" value={s.project} onChange={(e) => set("project", e.target.value)} />
          </Field>
          <div className="grid gap-2 sm:col-span-2">
            <div className="grid grid-cols-[1fr_7rem] gap-2 sm:grid-cols-[1fr_8rem_1fr_8rem]">
              <Field label={m.trip_departure()} htmlFor="startDate">
                <Input
                  id="startDate"
                  type="date"
                  value={s.startDate}
                  onChange={(e) =>
                    setS({
                      ...s,
                      startDate: e.target.value,
                      endDate: s.endDate && s.endDate >= e.target.value ? s.endDate : e.target.value,
                    })
                  }
                  required
                />
              </Field>
              <Field label={m.time()} htmlFor="startTime">
                <Input
                  id="startTime"
                  type="time"
                  value={s.startTime}
                  onChange={(e) => set("startTime", e.target.value)}
                  required
                />
              </Field>
              <Field label={m.trip_return()} htmlFor="endDate">
                <Input
                  id="endDate"
                  type="date"
                  min={s.startDate}
                  value={s.endDate}
                  onChange={(e) => set("endDate", e.target.value)}
                  required
                />
              </Field>
              <Field label={m.time()} htmlFor="endTime">
                <Input
                  id="endTime"
                  type="time"
                  value={s.endTime}
                  onChange={(e) => set("endTime", e.target.value)}
                  required
                />
              </Field>
            </div>
            <p className="flex gap-1.5 text-xs text-muted-foreground">
              <InfoIcon className="mt-0.5 size-3.5 shrink-0" />
              {m.trip_times_hint()}
            </p>
          </div>
          <label className="flex items-center gap-2 text-sm sm:col-span-2">
            <Checkbox checked={s.foreign} onChange={(e) => set("foreign", e.target.checked)} />
            {m.trip_foreign()}
          </label>
        </CardContent>
      </Card>

      {s.foreign ? (
        <Card>
          <CardHeader>
            <CardTitle>{m.trip_itinerary()}</CardTitle>
            <CardDescription>{m.trip_itinerary_hint()}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3">
            {s.stops.map((st, i) => {
              const cities = locations.filter((l) => l.country === st.country && l.city);
              const update = (patch: Partial<StopDto>) =>
                set(
                  "stops",
                  s.stops.map((x, j) => (j === i ? { ...x, ...patch } : x)),
                );
              return (
                <div
                  key={i}
                  className="grid gap-2 rounded-lg border p-3 sm:grid-cols-[1fr_1fr_1fr_auto] sm:items-end"
                >
                  <Field label={m.stop_arrival()}>
                    <Input
                      type="datetime-local"
                      value={st.arrival}
                      min={`${s.startDate}T${s.startTime}`}
                      max={`${s.endDate}T${s.endTime}`}
                      onChange={(e) => update({ arrival: e.target.value })}
                      required
                    />
                  </Field>
                  <Field label={m.country()}>
                    <Select
                      value={st.country}
                      onChange={(e) => update({ country: e.target.value, city: "" })}
                      required
                    >
                      <option value="">{m.choose()}</option>
                      <option value="DE">{m.germany()}</option>
                      {countries.map(([iso, label]) => (
                        <option key={iso} value={iso}>
                          {label}
                        </option>
                      ))}
                    </Select>
                  </Field>
                  <Field label={m.city_optional()}>
                    <Select
                      value={st.city ?? ""}
                      onChange={(e) => update({ city: e.target.value })}
                      disabled={!cities.length}
                    >
                      <option value="">{m.city_other()}</option>
                      {cities.map((c) => (
                        <option key={c.city} value={c.city}>
                          {c.city}
                        </option>
                      ))}
                    </Select>
                  </Field>
                  <div className="flex items-center justify-between gap-3 sm:justify-end">
                    <label className="flex items-center gap-2 text-sm">
                      <Checkbox
                        checked={st.business ?? true}
                        onChange={(e) => update({ business: e.target.checked })}
                      />
                      {m.stop_business()}
                    </label>
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={m.remove()}
                      onClick={() =>
                        set(
                          "stops",
                          s.stops.filter((_, j) => j !== i),
                        )
                      }
                    >
                      <Trash2Icon />
                    </Button>
                  </div>
                </div>
              );
            })}
            <Button
              variant="outline"
              className="justify-self-start"
              onClick={() =>
                set("stops", [
                  ...s.stops,
                  {
                    arrival: s.stops.at(-1)?.arrival ?? `${s.startDate}T${s.startTime}`,
                    tz: s.tz,
                    country: "",
                    city: "",
                    business: true,
                  },
                ])
              }
            >
              <PlusIcon />
              {m.add_stop()}
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {nightDates.length > 0 && !tooLong ? (
        <Card>
          <CardHeader>
            <CardTitle>{m.lodging_title()}</CardTitle>
            <CardDescription>{m.lodging_hint()}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3">
            {nightDates.length > 1 ? (
              <div className="flex flex-wrap items-center gap-2 text-sm">
                <span className="text-muted-foreground">{m.set_all_nights()}</span>
                <Select
                  className="w-auto"
                  value=""
                  onChange={(e) =>
                    e.target.value &&
                    set("nights", Object.fromEntries(nightDates.map((d) => [d, e.target.value])))
                  }
                >
                  <option value="">{m.choose()}</option>
                  {nightKinds().map((k) => (
                    <option key={k.value} value={k.value}>
                      {k.label}
                    </option>
                  ))}
                </Select>
              </div>
            ) : null}
            {nightDates.map((d) => (
              <div key={d} className="grid gap-1.5 sm:grid-cols-[12rem_1fr] sm:items-center">
                <Label htmlFor={`night-${d}`}>{m.night_of({ date: formatWeekday(d) })}</Label>
                <Select
                  id={`night-${d}`}
                  value={s.nights[d] ?? ""}
                  onChange={(e) => set("nights", { ...s.nights, [d]: e.target.value })}
                  aria-invalid={!s.nights[d]}
                  required
                >
                  <option value="">{m.choose()}</option>
                  {nightKinds().map((k) => (
                    <option key={k.value} value={k.value}>
                      {k.label}
                    </option>
                  ))}
                </Select>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : null}

      {range.length > 0 && !tooLong ? (
        <Card>
          <CardHeader>
            <CardTitle>{m.meals_title()}</CardTitle>
            <CardDescription>{m.meals_hint()}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-1">
            <div className="hidden grid-cols-[1fr_repeat(3,5.5rem)] gap-2 pb-1 text-xs font-medium text-muted-foreground sm:grid">
              <span />
              <span className="text-center">{m.breakfast()}</span>
              <span className="text-center">{m.lunch()}</span>
              <span className="text-center">{m.dinner()}</span>
            </div>
            {range.map((d) => (
              <DayRow key={d} day={day(d)} onChange={(p) => setDay(d, p)} />
            ))}
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{m.mileage_title()}</CardTitle>
          <CardDescription>{m.mileage_hint()}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3">
          {s.journeys.map((j, i) => {
            const update = (patch: Partial<FormState["journeys"][number]>) =>
              set(
                "journeys",
                s.journeys.map((x, k) => (k === i ? { ...x, ...patch } : x)),
              );
            return (
              <div
                key={i}
                className="grid gap-2 rounded-lg border p-3 sm:grid-cols-[9rem_10rem_8rem_1fr_auto] sm:items-end"
              >
                <Field label={m.date()}>
                  <Input
                    type="date"
                    value={j.date}
                    min={s.startDate}
                    max={s.endDate}
                    onChange={(e) => update({ date: e.target.value })}
                    required
                  />
                </Field>
                <Field label={m.vehicle()}>
                  <Select
                    value={j.vehicle}
                    onChange={(e) => update({ vehicle: e.target.value as JourneyDto["vehicle"] })}
                  >
                    <option value="car">{m.vehicle_car()}</option>
                    <option value="other_motor_vehicle">{m.vehicle_other()}</option>
                  </Select>
                </Field>
                <Field label={j.roundTrip ? m.km_one_way() : m.km_total()}>
                  <Input
                    inputMode="decimal"
                    value={j.km}
                    onChange={(e) => update({ km: e.target.value })}
                    required
                  />
                </Field>
                <Field label={m.description_optional()}>
                  <Input
                    value={j.description ?? ""}
                    onChange={(e) => update({ description: e.target.value })}
                  />
                </Field>
                <div className="flex items-center justify-between gap-2">
                  <label className="flex items-center gap-2 text-sm whitespace-nowrap">
                    <Checkbox
                      checked={j.roundTrip}
                      onChange={(e) => update({ roundTrip: e.target.checked })}
                    />
                    {m.round_trip()}
                  </label>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={m.remove()}
                    onClick={() =>
                      set(
                        "journeys",
                        s.journeys.filter((_, k) => k !== i),
                      )
                    }
                  >
                    <Trash2Icon />
                  </Button>
                </div>
              </div>
            );
          })}
          <Button
            variant="outline"
            className="justify-self-start"
            onClick={() =>
              set("journeys", [
                ...s.journeys,
                { date: s.startDate, vehicle: "car", distanceMeters: 0, km: "", roundTrip: true },
              ])
            }
          >
            <PlusIcon />
            {m.add_journey()}
          </Button>
        </CardContent>
      </Card>

      <details className="group rounded-xl border bg-card">
        <summary className="flex cursor-pointer list-none items-center justify-between px-4 py-3 text-sm font-medium sm:px-5">
          {m.advanced()}
          <ChevronDownIcon className="size-4 transition-transform group-open:rotate-180" />
        </summary>
        <div className="grid gap-4 border-t px-4 py-4 sm:px-5">
          <Field label={m.notes()} htmlFor="notes">
            <Input id="notes" value={s.notes} onChange={(e) => set("notes", e.target.value)} />
          </Field>
          <Field label={m.time_zone()} htmlFor="tz" hint={m.time_zone_hint()}>
            <Input id="tz" value={s.tz} onChange={(e) => set("tz", e.target.value)} required />
          </Field>
          {range.length > 0 && !tooLong ? (
            <div className="grid gap-2">
              <Label>{m.exclude_days()}</Label>
              <p className="text-xs text-muted-foreground">{m.exclude_days_hint()}</p>
              {range.map((d) => (
                <div key={d} className="grid gap-2 sm:grid-cols-[12rem_1fr] sm:items-center">
                  <label className="flex items-center gap-2 text-sm">
                    <Checkbox
                      checked={!!day(d).excluded}
                      onChange={(e) => setDay(d, { excluded: e.target.checked })}
                    />
                    {formatWeekday(d)}
                  </label>
                  {day(d).excluded ? (
                    <Input
                      placeholder={m.reason()}
                      value={day(d).excludeReason ?? ""}
                      onChange={(e) => setDay(d, { excludeReason: e.target.value })}
                      required
                    />
                  ) : null}
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </details>

      <div className="sticky bottom-20 z-10 flex justify-end md:bottom-4">
        <Button type="submit" size="lg" disabled={pending} className="shadow-lg">
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}

function DayRow({ day, onChange }: { day: DayDto; onChange: (p: Partial<DayDto>) => void }) {
  const [showContrib, setShowContrib] = useState(
    !!(day.contribBreakfast || day.contribLunch || day.contribDinner),
  );
  const meal = (key: "breakfast" | "lunch" | "dinner", label: string) => (
    <label className="flex items-center justify-center gap-2 rounded-md border px-2 py-2 text-sm has-[:checked]:border-primary has-[:checked]:bg-accent sm:border-0 sm:py-1.5 sm:has-[:checked]:bg-transparent">
      <Checkbox
        checked={!!day[key]}
        onChange={(e) =>
          onChange({
            [key]: e.target.checked,
            ...(key === "breakfast" && !e.target.checked ? { breakfastOrigin: "" } : {}),
          })
        }
      />
      <span className="sm:sr-only">{label}</span>
    </label>
  );
  const contrib = (key: "contribBreakfast" | "contribLunch" | "contribDinner", label: string) => (
    <Field label={label}>
      <Input
        inputMode="decimal"
        placeholder="0,00"
        defaultValue={minorToInput(day[key] || null)}
        onBlur={(e) => onChange({ [key]: parseAmount(e.target.value) ?? 0 })}
      />
    </Field>
  );
  return (
    <div className="grid gap-2 border-b py-2 last:border-0">
      <div className="grid grid-cols-3 items-center gap-2 sm:grid-cols-[1fr_repeat(3,5.5rem)]">
        <div className="col-span-3 flex items-center justify-between gap-2 sm:col-span-1">
          <span className="text-sm font-medium">{formatWeekday(day.date)}</span>
          <span className="flex items-center gap-2">
            {day.breakfastOrigin ? (
              <span className="text-xs text-muted-foreground">{m.breakfast_from_hotel()}</span>
            ) : null}
            <button
              type="button"
              className="text-xs text-primary hover:underline"
              onClick={() => setShowContrib(!showContrib)}
            >
              {m.contribution()}
            </button>
          </span>
        </div>
        {meal("breakfast", m.breakfast())}
        {meal("lunch", m.lunch())}
        {meal("dinner", m.dinner())}
      </div>
      {showContrib ? (
        <div className="grid grid-cols-3 gap-2">
          {contrib("contribBreakfast", m.breakfast())}
          {contrib("contribLunch", m.lunch())}
          {contrib("contribDinner", m.dinner())}
        </div>
      ) : null}
    </div>
  );
}
