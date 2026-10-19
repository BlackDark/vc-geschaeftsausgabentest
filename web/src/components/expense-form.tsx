import { useQuery } from "@tanstack/react-query";
import { ChevronDownIcon, FileIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useState } from "react";
import { listReceiptsOptions } from "@/api/gen/@tanstack/react-query.gen";
import type { Expense, ExpenseInput, Receipt, VatPortion } from "@/api/gen/types.gen";
import { costTypeLabel } from "@/components/common";
import { ReceiptCapture, ReceiptFilePicker } from "@/components/receipt-capture";
import { SelfIssuedDialog } from "@/components/self-issued";
import { Button } from "@/components/ui/button";
import { Checkbox, Field, Input, Label, Select, Textarea } from "@/components/ui/input";
import { Alert } from "@/components/ui/misc";
import { formatDate, formatWeekday, minorToInput, parseAmount, todayISO } from "@/lib/format";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

const travelTypes = ["transport", "lodging", "meals", "incidental", "hospitality"] as const;
const outlayTypes = ["tools_equipment", "software_services", "other_outlay"] as const;
const exponents: Record<string, number> = {
  JPY: 0,
  KRW: 0,
  ISK: 0,
  CLP: 0,
  VND: 0,
  BHD: 3,
  KWD: 3,
  OMR: 3,
  JOD: 3,
  TND: 3,
};
const currencies = [
  "EUR",
  "USD",
  "GBP",
  "CHF",
  "PLN",
  "CZK",
  "DKK",
  "SEK",
  "NOK",
  "HUF",
  "JPY",
  "CNY",
  "CAD",
  "AUD",
];

export function exponentOf(currency: string) {
  return exponents[currency] ?? 2;
}

type VatRow = { rate: string; amount: string };

export function ExpenseForm({
  tripId,
  nights = [],
  tripRange,
  expense,
  pending,
  onSubmit,
}: {
  tripId?: number;
  nights?: string[];
  tripRange?: { start: string; end: string };
  expense?: Expense;
  pending?: boolean;
  onSubmit: (input: ExpenseInput) => void;
}) {
  const types = tripId ? travelTypes : outlayTypes;
  const [costType, setCostType] = useState<ExpenseInput["costType"]>(expense?.costType ?? types[0]);
  const [date, setDate] = useState(expense?.date ?? tripRange?.start ?? todayISO());
  const [payer, setPayer] = useState<ExpenseInput["payer"]>(expense?.payer ?? "user");
  const [currency, setCurrency] = useState(expense?.currency ?? "EUR");
  const exp = exponentOf(currency);
  const [amount, setAmount] = useState(
    expense ? minorToInput(expense.grossMinor, exponentOf(expense.currency)) : "",
  );
  const [fxMode, setFxMode] = useState<"rate" | "debit">(expense?.eurDebit != null ? "debit" : "rate");
  const [fxRate, setFxRate] = useState(expense?.fxRate ?? "");
  const [fxDate, setFxDate] = useState(expense?.fxDate ?? "");
  const [debit, setDebit] = useState(minorToInput(expense?.eurDebit));
  const [supplier, setSupplier] = useState(expense?.supplier ?? "");
  const [description, setDescription] = useState(expense?.description ?? "");
  const [covered, setCovered] = useState<string[]>(expense?.nights ?? nights);
  const [breakfast, setBreakfast] = useState(expense?.includesBreakfast ?? false);
  const [fuel, setFuel] = useState(expense?.fuel ?? false);
  const [receiptIds, setReceiptIds] = useState<number[]>(expense?.receiptIds ?? []);
  const [vatRows, setVatRows] = useState<VatRow[]>(
    (expense?.vat ?? []).map((v) => ({
      rate: String(v.rateBp / 100),
      amount: minorToInput(v.grossMinor, exponentOf(expense?.currency ?? "EUR")),
    })),
  );
  const h = expense?.hospitality;
  const [occasion, setOccasion] = useState(h?.occasion ?? "");
  const [place, setPlace] = useState(h?.place ?? "");
  const [participants, setParticipants] = useState((h?.participants ?? []).join("\n"));
  const [external, setExternal] = useState(String(h?.externalCount ?? 1));
  const [confirmed, setConfirmed] = useState(h?.confirmed ?? false);
  const [error, setError] = useState<string | null>(null);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const gross = parseAmount(amount, exp);
    if (!gross || gross <= 0) return setError(m.amount_invalid());
    const vat: VatPortion[] = [];
    for (const r of vatRows) {
      const g = parseAmount(r.amount, exp);
      const rate = Number(r.rate.replace(",", "."));
      if (g == null || !Number.isFinite(rate)) return setError(m.vat_invalid());
      vat.push({ grossMinor: g, rateBp: Math.round(rate * 100), country: "DE" });
    }
    if (vat.length && vat.reduce((a, v) => a + v.grossMinor, 0) !== gross)
      return setError(m.vat_sum_mismatch());
    let eurDebit: number | undefined;
    if (currency !== "EUR" && fxMode === "debit") {
      eurDebit = parseAmount(debit) ?? undefined;
      if (!eurDebit) return setError(m.amount_invalid());
    }
    setError(null);
    onSubmit({
      tripId,
      date,
      costType,
      payer: tripId ? payer : "user",
      supplier: supplier || undefined,
      description: description || undefined,
      currency,
      grossMinor: gross,
      fxRate: currency !== "EUR" && fxMode === "rate" ? fxRate : undefined,
      fxDate: currency !== "EUR" && fxDate ? fxDate : undefined,
      eurDebit,
      vatUnknown: vat.length === 0,
      vat,
      nights: costType === "lodging" ? covered : undefined,
      includesBreakfast: costType === "lodging" ? breakfast : undefined,
      fuel: costType === "transport" ? fuel : undefined,
      hospitality:
        costType === "hospitality"
          ? {
              occasion,
              place,
              participants: participants
                .split("\n")
                .map((p) => p.trim())
                .filter(Boolean),
              externalCount: Number(external) || 0,
              confirmed,
            }
          : undefined,
      receiptIds,
      version: expense?.version,
    });
  };

  return (
    <form className="grid gap-4" onSubmit={submit}>
      <ReceiptPicker selected={receiptIds} onChange={setReceiptIds} />

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label={m.cost_type()} htmlFor="costType">
          <Select
            id="costType"
            value={costType}
            onChange={(e) => setCostType(e.target.value as ExpenseInput["costType"])}
          >
            {types.map((t) => (
              <option key={t} value={t}>
                {costTypeLabel(t)}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={m.date()} htmlFor="date">
          <Input id="date" type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
        </Field>
        <Field label={m.amount_gross()} htmlFor="amount">
          <div className="flex gap-2">
            <Input
              id="amount"
              inputMode="decimal"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              required
              placeholder={exp === 0 ? "0" : "0,00"}
              autoFocus={!expense}
            />
            <Select
              aria-label={m.currency()}
              className="w-24"
              value={currency}
              onChange={(e) => setCurrency(e.target.value)}
            >
              {currencies.map((c) => (
                <option key={c}>{c}</option>
              ))}
            </Select>
          </div>
        </Field>
        {tripId ? (
          <Field
            label={m.paid_by()}
            htmlFor="payer"
            hint={payer === "employer" ? m.paid_by_employer_hint() : undefined}
          >
            <Select
              id="payer"
              value={payer}
              onChange={(e) => setPayer(e.target.value as ExpenseInput["payer"])}
            >
              <option value="user">{m.payer_user()}</option>
              <option value="employer">{m.payer_employer()}</option>
            </Select>
          </Field>
        ) : null}
        <Field label={m.supplier()} htmlFor="supplier">
          <Input
            id="supplier"
            value={supplier}
            onChange={(e) => setSupplier(e.target.value)}
            placeholder={m.supplier_placeholder()}
          />
        </Field>
        <Field
          label={tripId ? m.description_optional() : m.outlay_purpose()}
          htmlFor="description"
          className={tripId ? "" : "sm:col-span-2"}
        >
          <Input
            id="description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            required={!tripId}
          />
        </Field>
      </div>

      {currency !== "EUR" ? (
        <div className="grid gap-3 rounded-lg border p-3">
          <Label>{m.fx_title()}</Label>
          <div className="flex gap-4 text-sm">
            <label className="flex items-center gap-2">
              <input type="radio" checked={fxMode === "rate"} onChange={() => setFxMode("rate")} />
              {m.fx_rate_mode()}
            </label>
            <label className="flex items-center gap-2">
              <input type="radio" checked={fxMode === "debit"} onChange={() => setFxMode("debit")} />
              {m.fx_debit_mode()}
            </label>
          </div>
          {fxMode === "rate" ? (
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label={m.fx_rate({ currency })} hint={m.fx_rate_hint()}>
                <Input
                  inputMode="decimal"
                  value={fxRate}
                  onChange={(e) => setFxRate(e.target.value)}
                  placeholder="1,1650"
                  required
                />
              </Field>
              <Field label={m.fx_date()}>
                <Input type="date" value={fxDate} onChange={(e) => setFxDate(e.target.value)} />
              </Field>
            </div>
          ) : (
            <Field label={m.fx_debit()} hint={m.fx_debit_hint()}>
              <Input inputMode="decimal" value={debit} onChange={(e) => setDebit(e.target.value)} required />
            </Field>
          )}
        </div>
      ) : null}

      {costType === "lodging" && nights.length ? (
        <div className="grid gap-2 rounded-lg border p-3">
          <Label>{m.nights_covered()}</Label>
          <div className="flex flex-wrap gap-2">
            {nights.map((n) => (
              <label
                key={n}
                className="flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-sm has-[:checked]:border-primary has-[:checked]:bg-accent"
              >
                <Checkbox
                  checked={covered.includes(n)}
                  onChange={(e) =>
                    setCovered(e.target.checked ? [...covered, n].sort() : covered.filter((x) => x !== n))
                  }
                />
                {formatWeekday(n)}
              </label>
            ))}
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={breakfast} onChange={(e) => setBreakfast(e.target.checked)} />
            {m.includes_breakfast()}
          </label>
          {breakfast ? <p className="text-xs text-muted-foreground">{m.includes_breakfast_hint()}</p> : null}
        </div>
      ) : null}

      {costType === "transport" ? (
        <label className="flex items-center gap-2 text-sm">
          <Checkbox checked={fuel} onChange={(e) => setFuel(e.target.checked)} />
          {m.fuel_receipt()}
        </label>
      ) : null}

      {costType === "hospitality" ? (
        <div className="grid gap-3 rounded-lg border p-3 sm:grid-cols-2">
          <Field label={m.hosp_occasion()}>
            <Input value={occasion} onChange={(e) => setOccasion(e.target.value)} required />
          </Field>
          <Field label={m.hosp_place()}>
            <Input value={place} onChange={(e) => setPlace(e.target.value)} required />
          </Field>
          <Field label={m.hosp_participants()} hint={m.hosp_participants_hint()} className="sm:col-span-2">
            <Textarea
              value={participants}
              onChange={(e) => setParticipants(e.target.value)}
              required
              rows={3}
            />
          </Field>
          <Field label={m.hosp_external()}>
            <Input
              type="number"
              min={1}
              value={external}
              onChange={(e) => setExternal(e.target.value)}
              required
            />
          </Field>
          <label className="flex items-center gap-2 self-end pb-2 text-sm">
            <Checkbox checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} required />
            {m.hosp_confirm()}
          </label>
        </div>
      ) : null}

      <details className="group rounded-lg border" open={vatRows.length > 0}>
        <summary className="flex cursor-pointer list-none items-center justify-between px-3 py-2.5 text-sm font-medium">
          {m.vat_title()}
          <span className="flex items-center gap-2 text-xs font-normal text-muted-foreground">
            {vatRows.length ? m.vat_n_rates({ count: String(vatRows.length) }) : m.vat_unknown()}
            <ChevronDownIcon className="size-4 transition-transform group-open:rotate-180" />
          </span>
        </summary>
        <div className="grid gap-2 border-t p-3">
          <p className="text-xs text-muted-foreground">{m.vat_hint()}</p>
          {vatRows.map((r, i) => (
            <div key={i} className="grid grid-cols-[6rem_1fr_auto] items-end gap-2">
              <Field label={m.vat_rate()}>
                <Select
                  value={r.rate}
                  onChange={(e) =>
                    setVatRows(vatRows.map((x, j) => (j === i ? { ...x, rate: e.target.value } : x)))
                  }
                >
                  <option value="19">19 %</option>
                  <option value="7">7 %</option>
                  <option value="0">0 %</option>
                </Select>
              </Field>
              <Field label={m.vat_gross_part()}>
                <Input
                  inputMode="decimal"
                  value={r.amount}
                  onChange={(e) =>
                    setVatRows(vatRows.map((x, j) => (j === i ? { ...x, amount: e.target.value } : x)))
                  }
                />
              </Field>
              <Button
                variant="ghost"
                size="icon"
                aria-label={m.remove()}
                onClick={() => setVatRows(vatRows.filter((_, j) => j !== i))}
              >
                <Trash2Icon />
              </Button>
            </div>
          ))}
          <Button
            variant="outline"
            size="sm"
            className="justify-self-start"
            onClick={() =>
              setVatRows([...vatRows, { rate: "19", amount: vatRows.length === 0 ? amount : "" }])
            }
          >
            <PlusIcon />
            {m.vat_add()}
          </Button>
        </div>
      </details>

      {error ? <Alert variant="error">{error}</Alert> : null}
      <Button type="submit" size="lg" disabled={pending}>
        {m.save()}
      </Button>
    </form>
  );
}

/** Lets the user attach existing inbox/confirmed receipts or capture new ones. */
function ReceiptPicker({ selected, onChange }: { selected: number[]; onChange: (ids: number[]) => void }) {
  const { data: receipts = [] } = useQuery(listReceiptsOptions({ query: {} }));
  const usable = receipts.filter((r) => r.status !== "void");
  const chosen = usable.filter((r) => selected.includes(r.id));
  const available = usable.filter((r) => !selected.includes(r.id) && r.expenses.length === 0);
  const add = (ids: number[]) => onChange([...new Set([...selected, ...ids])]);
  return (
    <div className="grid gap-2">
      <Label>{m.receipts()}</Label>
      <div className="flex flex-wrap gap-2">
        {chosen.map((r) => (
          <ReceiptChip
            key={r.id}
            receipt={r}
            selected
            onClick={() => onChange(selected.filter((x) => x !== r.id))}
          />
        ))}
        {available.slice(0, 8).map((r) => (
          <ReceiptChip key={r.id} receipt={r} onClick={() => add([r.id])} />
        ))}
      </div>
      <div className="flex flex-wrap gap-2">
        <ReceiptCapture variant="outline" onUploaded={add} />
        <ReceiptFilePicker onUploaded={add} />
        <SelfIssuedDialog onCreated={(id) => add([id])} />
      </div>
      {chosen.some((r) => r.status === "inbox") ? (
        <p className="text-xs text-muted-foreground">{m.receipt_confirm_later()}</p>
      ) : null}
    </div>
  );
}

function ReceiptChip({
  receipt,
  selected,
  onClick,
}: {
  receipt: Receipt;
  selected?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={cn(
        "flex items-center gap-2 rounded-lg border p-1.5 pr-3 text-left text-xs transition-colors",
        selected ? "border-primary bg-accent" : "border-dashed opacity-80 hover:opacity-100",
      )}
    >
      {receipt.hasPreview ? (
        <img
          src={`/api/receipts/${receipt.id}/preview`}
          alt=""
          className="size-10 rounded object-cover object-top"
          loading="lazy"
        />
      ) : (
        <span className="grid size-10 place-items-center rounded bg-muted">
          <FileIcon className="size-4" />
        </span>
      )}
      <span className="grid">
        <span className="font-medium">{receipt.number || m.receipt_unconfirmed()}</span>
        <span className="max-w-32 truncate text-muted-foreground">
          {receipt.filename || formatDate(receipt.createdAt)}
        </span>
      </span>
      {selected ? <span className="text-primary">✓</span> : <PlusIcon className="size-3.5" />}
    </button>
  );
}
