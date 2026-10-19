import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { KeyRoundIcon, LinkIcon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
  changePasswordMutation,
  getEmployerOptions,
  listAuditOptions,
  saveEmployerMutation,
  updateMeMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import type { ProfileInput } from "@/api/gen/types.gen";
import { PageHeader } from "@/components/common";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { Alert, Skeleton } from "@/components/ui/misc";
import { errorText } from "@/lib/api";
import { sessionQuery, useSession, useUser } from "@/lib/session";
import { applyTheme, type Theme } from "@/lib/theme";
import { m } from "@/paraglide/messages";
import { getLocale, setLocale } from "@/paraglide/runtime";

type Search = { welcome?: boolean; linked?: boolean; error?: string };

export const Route = createFileRoute("/_app/settings")({
  validateSearch: (s: Record<string, unknown>): Search => ({
    welcome: s.welcome === true || s.welcome === "true" ? true : undefined,
    linked: s.linked === 1 || s.linked === "1" ? true : undefined,
    error: typeof s.error === "string" ? s.error : undefined,
  }),
  component: Settings,
});

function Settings() {
  const search = Route.useSearch();
  return (
    <>
      <PageHeader title={m.nav_settings()} />
      <div className="grid gap-4">
        {search.welcome ? <Alert title={m.welcome_title()}>{m.welcome_text()}</Alert> : null}
        {search.linked ? <Alert title={m.oidc_linked()} /> : null}
        {search.error ? <Alert variant="error">{errorText({ code: search.error })}</Alert> : null}
        <EmployerCard />
        <ProfileCard />
        <SecurityCard />
        <AuditCard />
      </div>
    </>
  );
}

function EmployerCard() {
  const qc = useQueryClient();
  const { data: wrapper, isLoading } = useQuery(getEmployerOptions());
  const data = wrapper?.employer;
  const save = useMutation({
    ...saveEmployerMutation(),
    onSuccess: (e) => {
      qc.setQueryData(getEmployerOptions().queryKey, { employer: e });
      qc.invalidateQueries({ queryKey: [{ _id: "getDashboard" }] });
      toast.success(m.saved());
    },
  });
  if (isLoading) return <Skeleton className="h-48" />;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.employer()}</CardTitle>
        <CardDescription>{m.employer_hint()}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          key={data?.version ?? 0}
          className="grid gap-4 sm:grid-cols-2"
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            save.mutate({
              body: {
                name: String(f.get("name")),
                address: String(f.get("address")),
                version: data?.version,
              },
            });
          }}
        >
          <Field label={m.employer_name()} htmlFor="emp-name">
            <Input id="emp-name" name="name" defaultValue={data?.name} required />
          </Field>
          <Field label={m.address()} htmlFor="emp-address">
            <Textarea id="emp-address" name="address" defaultValue={data?.address} rows={2} />
          </Field>
          <div className="sm:col-span-2">
            <Button type="submit" disabled={save.isPending}>
              {m.save()}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function ProfileCard() {
  const user = useUser();
  const qc = useQueryClient();
  const [p, setP] = useState<ProfileInput>({
    name: user.name,
    email: user.email,
    locale: user.locale as ProfileInput["locale"],
    theme: user.theme as ProfileInput["theme"],
    employeeNumber: user.employeeNumber,
    address: user.address,
    iban: user.iban,
  });
  const save = useMutation({
    ...updateMeMutation(),
    onSuccess: (u) => {
      qc.setQueryData(sessionQuery.queryKey, (s) => (s ? { ...s, user: u } : s));
      applyTheme(u.theme as Theme);
      toast.success(m.saved());
      if (u.locale !== getLocale()) setLocale(u.locale as "de" | "en");
    },
  });
  const set =
    (k: keyof ProfileInput) =>
    (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) =>
      setP({ ...p, [k]: e.target.value });
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.profile()}</CardTitle>
        <CardDescription>{m.profile_hint()}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className="grid gap-4 sm:grid-cols-2"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate({ body: p });
          }}
        >
          <Field label={m.your_name()} htmlFor="name">
            <Input id="name" value={p.name} onChange={set("name")} required autoComplete="name" />
          </Field>
          <Field label={m.email()} htmlFor="email">
            <Input
              id="email"
              type="email"
              value={p.email ?? ""}
              onChange={set("email")}
              autoComplete="email"
            />
          </Field>
          <Field label={m.employee_number()} htmlFor="empno">
            <Input id="empno" value={p.employeeNumber ?? ""} onChange={set("employeeNumber")} />
          </Field>
          <Field label={m.iban()} htmlFor="iban" hint={m.iban_hint()}>
            <Input id="iban" value={p.iban ?? ""} onChange={set("iban")} autoComplete="off" />
          </Field>
          <Field label={m.address()} htmlFor="address" className="sm:col-span-2">
            <Textarea id="address" value={p.address ?? ""} onChange={set("address")} rows={2} />
          </Field>
          <Field label={m.language()} htmlFor="locale">
            <Select id="locale" value={p.locale} onChange={set("locale")}>
              <option value="de">Deutsch</option>
              <option value="en">English</option>
            </Select>
          </Field>
          <Field label={m.theme()} htmlFor="theme">
            <Select id="theme" value={p.theme} onChange={set("theme")}>
              <option value="dark">{m.theme_dark()}</option>
              <option value="light">{m.theme_light()}</option>
              <option value="system">{m.theme_system()}</option>
            </Select>
          </Field>
          <div className="sm:col-span-2">
            <Button type="submit" disabled={save.isPending}>
              {m.save()}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function SecurityCard() {
  const user = useUser();
  const { data: session } = useSession();
  const [pw, setPw] = useState({ current: "", next: "" });
  const change = useMutation({
    ...changePasswordMutation(),
    onSuccess: () => {
      setPw({ current: "", next: "" });
      toast.success(m.password_changed());
    },
  });
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.security()}</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-5">
        {session?.oidcEnabled ? (
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="grid gap-0.5">
              <span className="text-sm font-medium">{session.oidcLabel}</span>
              <span className="text-xs text-muted-foreground">
                {user.oidcLinked ? m.oidc_is_linked() : m.oidc_not_linked()}
              </span>
            </div>
            <Button variant="outline" onClick={() => window.location.assign("/auth/oidc/login?link=1")}>
              <LinkIcon />
              {user.oidcLinked ? m.oidc_relink() : m.oidc_link()}
            </Button>
          </div>
        ) : null}
        {session?.passwordEnabled ? (
          <form
            className="grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end"
            onSubmit={(e) => {
              e.preventDefault();
              change.mutate({ body: pw });
            }}
          >
            <Field label={m.current_password()}>
              <Input
                type="password"
                autoComplete="current-password"
                value={pw.current}
                onChange={(e) => setPw({ ...pw, current: e.target.value })}
              />
            </Field>
            <Field label={m.new_password()}>
              <Input
                type="password"
                autoComplete="new-password"
                minLength={10}
                value={pw.next}
                onChange={(e) => setPw({ ...pw, next: e.target.value })}
                required
              />
            </Field>
            <Button type="submit" variant="outline" disabled={change.isPending}>
              <KeyRoundIcon />
              {m.change_password()}
            </Button>
          </form>
        ) : null}
      </CardContent>
    </Card>
  );
}

function AuditCard() {
  const { data } = useQuery(listAuditOptions({ query: { limit: 50 } }));
  return (
    <details className="group rounded-xl border bg-card">
      <summary className="cursor-pointer px-5 py-4 text-sm font-medium">{m.audit_trail()}</summary>
      <div className="max-h-96 overflow-auto border-t px-5 py-3">
        <ul className="grid gap-1.5 text-xs">
          {(data ?? []).map((e) => (
            <li key={e.id} className="grid grid-cols-[9rem_1fr] gap-2">
              <span className="text-muted-foreground tabular">{e.at.replace("T", " ").slice(0, 16)}</span>
              <span>
                {e.entity} #{e.entityId} · {e.action}
                {e.reason ? ` · ${e.reason}` : ""}
              </span>
            </li>
          ))}
        </ul>
      </div>
    </details>
  );
}
