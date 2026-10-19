import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FilePenIcon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { createSelfIssuedMutation } from "@/api/gen/@tanstack/react-query.gen";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Field, Input, Textarea } from "@/components/ui/input";
import { parseAmount, todayISO } from "@/lib/format";
import { useUser } from "@/lib/session";
import { m } from "@/paraglide/messages";

/** Eigenbeleg: a self-issued receipt when the original is missing (P11). */
export function SelfIssuedDialog({ onCreated }: { onCreated?: (id: number) => void }) {
  const user = useUser();
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [f, setF] = useState({ date: todayISO(), amount: "", payee: "", purpose: "", reason: "" });
  const create = useMutation({
    ...createSelfIssuedMutation(),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: [{ _id: "listReceipts" }] });
      toast.success(m.self_issued_created({ number: r.number }));
      setOpen(false);
      onCreated?.(r.id);
    },
  });
  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setF({ ...f, [k]: e.target.value });
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button variant="ghost" onClick={() => setOpen(true)}>
        <FilePenIcon />
        {m.self_issued()}
      </Button>
      <DialogContent title={m.self_issued()} description={m.self_issued_hint()}>
        <form
          className="grid gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate({
              body: {
                date: f.date,
                amountMinor: parseAmount(f.amount) ?? 0,
                currency: "EUR",
                payee: f.payee,
                purpose: f.purpose,
                issuer: user.name,
                reason: f.reason,
              },
            });
          }}
        >
          <div className="grid grid-cols-2 gap-3">
            <Field label={m.date()}>
              <Input type="date" value={f.date} onChange={set("date")} required />
            </Field>
            <Field label={m.amount_gross()}>
              <Input inputMode="decimal" value={f.amount} onChange={set("amount")} required />
            </Field>
          </div>
          <Field label={m.self_issued_payee()}>
            <Input value={f.payee} onChange={set("payee")} required />
          </Field>
          <Field label={m.self_issued_purpose()}>
            <Input value={f.purpose} onChange={set("purpose")} required />
          </Field>
          <Field label={m.self_issued_reason()}>
            <Textarea value={f.reason} onChange={set("reason")} required rows={2} />
          </Field>
          <Button type="submit" disabled={create.isPending}>
            {m.self_issued_create()}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}
