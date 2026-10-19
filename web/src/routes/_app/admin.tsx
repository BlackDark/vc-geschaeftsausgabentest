import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { UserPlusIcon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
  createUserMutation,
  listUsersOptions,
  setUserActiveMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import { PageHeader } from "@/components/common";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox, Field, Input } from "@/components/ui/input";
import { useUser } from "@/lib/session";
import { m } from "@/paraglide/messages";

export const Route = createFileRoute("/_app/admin")({ component: Admin });

function Admin() {
  const me = useUser();
  const qc = useQueryClient();
  const { data = [] } = useQuery({ ...listUsersOptions(), enabled: me.admin });
  const refresh = () => qc.invalidateQueries({ queryKey: [{ _id: "listUsers" }] });
  const [f, setF] = useState({ username: "", name: "", password: "", admin: false });
  const create = useMutation({
    ...createUserMutation(),
    onSuccess: () => {
      refresh();
      setF({ username: "", name: "", password: "", admin: false });
      toast.success(m.user_created());
    },
  });
  const setActive = useMutation({ ...setUserActiveMutation(), onSuccess: refresh });
  return (
    <>
      <PageHeader title={m.nav_admin()} description={m.admin_hint()} />
      <div className="grid gap-4">
        <Card>
          <CardHeader>
            <CardTitle>{m.users()}</CardTitle>
          </CardHeader>
          <CardContent className="pt-2">
            <ul className="divide-y">
              {data.map((u) => (
                <li key={u.id} className="flex flex-wrap items-center justify-between gap-3 py-2.5">
                  <span className="grid">
                    <span className="flex items-center gap-2 text-sm font-medium">
                      {u.name}
                      {u.admin ? <Badge>{m.admin()}</Badge> : null}
                      {!u.active ? <Badge variant="destructive">{m.inactive()}</Badge> : null}
                      {u.oidcLinked ? <Badge variant="outline">OIDC</Badge> : null}
                    </span>
                    <span className="text-xs text-muted-foreground">{u.username}</span>
                  </span>
                  {u.id !== me.id ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setActive.mutate({ path: { id: u.id }, body: { active: !u.active } })}
                    >
                      {u.active ? m.deactivate() : m.activate()}
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{m.create_user()}</CardTitle>
            <CardDescription>{m.create_user_hint()}</CardDescription>
          </CardHeader>
          <CardContent>
            <form
              className="grid gap-3 sm:grid-cols-3 sm:items-end"
              onSubmit={(e) => {
                e.preventDefault();
                create.mutate({ body: f });
              }}
            >
              <Field label={m.your_name()}>
                <Input value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} required />
              </Field>
              <Field label={m.username()}>
                <Input
                  value={f.username}
                  onChange={(e) => setF({ ...f, username: e.target.value })}
                  required
                  autoComplete="off"
                />
              </Field>
              <Field label={m.initial_password()}>
                <Input
                  type="password"
                  minLength={10}
                  value={f.password}
                  onChange={(e) => setF({ ...f, password: e.target.value })}
                  required
                  autoComplete="new-password"
                />
              </Field>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox checked={f.admin} onChange={(e) => setF({ ...f, admin: e.target.checked })} />
                {m.admin()}
              </label>
              <Button type="submit" className="sm:col-start-3" disabled={create.isPending}>
                <UserPlusIcon />
                {m.create_user()}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
