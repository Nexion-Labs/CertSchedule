import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type FormEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { createDomain, getDomain, updateDomain } from "../api/client";
import type { ChallengeType, DNSProvider, DomainInput } from "../api/types";

const emptyForm: DomainInput = {
  name: "",
  challenge_type: "http-01",
  dns_provider: "",
  k8s_namespace: "default",
  k8s_secret_name: "",
  auto_renew: true,
  renew_before_days: 3,
  cloudflare_api_token: "",
  route53_access_key_id: "",
  route53_secret_access_key: "",
};

export function DomainFormPage() {
  const { id } = useParams();
  const isEdit = !!id;
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [form, setForm] = useState<DomainInput>(emptyForm);
  const [error, setError] = useState<string | null>(null);

  const { data: existing } = useQuery({
    queryKey: ["domain", id],
    queryFn: () => getDomain(id as string),
    enabled: isEdit,
  });

  useEffect(() => {
    if (existing) {
      setForm((f) => ({
        ...f,
        name: existing.name,
        challenge_type: existing.challenge_type,
        dns_provider: existing.dns_provider,
        k8s_namespace: existing.k8s_namespace,
        k8s_secret_name: existing.k8s_secret_name,
        auto_renew: existing.auto_renew,
        renew_before_days: existing.renew_before_days,
      }));
    }
  }, [existing]);

  const mutation = useMutation({
    mutationFn: (input: DomainInput) => (isEdit ? updateDomain(id as string, input) : createDomain(input)),
    onSuccess: (d) => {
      queryClient.invalidateQueries({ queryKey: ["domains"] });
      navigate(`/domains/${d.id}`);
    },
    onError: () => setError("Failed to save domain. Check the fields and try again."),
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    mutation.mutate(form);
  }

  return (
    <div className="mx-auto max-w-lg space-y-4">
      <h1 className="text-xl font-semibold">{isEdit ? "Edit domain" : "New domain"}</h1>

      {error && (
        <div className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-300">
          {error}
        </div>
      )}

      <form onSubmit={onSubmit} className="space-y-4 rounded-lg border border-slate-200 p-4 dark:border-slate-800">
        <Field label="Domain name">
          <input
            className="input"
            placeholder="example.com or *.example.com"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            required
          />
        </Field>

        <Field label="Challenge type">
          <select
            className="input"
            value={form.challenge_type}
            onChange={(e) => setForm({ ...form, challenge_type: e.target.value as ChallengeType })}
            disabled={isEdit}
          >
            <option value="http-01">HTTP-01</option>
            <option value="dns-01">DNS-01</option>
          </select>
        </Field>

        {form.challenge_type === "dns-01" && (
          <Field label="DNS provider">
            <select
              className="input"
              value={form.dns_provider}
              onChange={(e) => setForm({ ...form, dns_provider: e.target.value as DNSProvider })}
              disabled={isEdit}
            >
              <option value="">Select a provider</option>
              <option value="cloudflare">Cloudflare</option>
              <option value="route53">AWS Route53</option>
            </select>
          </Field>
        )}

        {form.challenge_type === "dns-01" && form.dns_provider === "cloudflare" && (
          <Field label={isEdit ? "Cloudflare API token (leave blank to keep current)" : "Cloudflare API token"}>
            <input
              type="password"
              className="input"
              value={form.cloudflare_api_token}
              onChange={(e) => setForm({ ...form, cloudflare_api_token: e.target.value })}
              placeholder={isEdit ? "••••••••••••" : "e.g. AbCdEf123456_ScopedAPITokenValue"}
              required={!isEdit}
            />
          </Field>
        )}

        {form.challenge_type === "dns-01" && form.dns_provider === "route53" && (
          <>
            <Field label={isEdit ? "AWS access key ID (leave blank to keep current)" : "AWS access key ID"}>
              <input
                className="input"
                value={form.route53_access_key_id}
                onChange={(e) => setForm({ ...form, route53_access_key_id: e.target.value })}
                placeholder={isEdit ? "••••••••••••" : "AKIAIOSFODNN7EXAMPLE"}
                required={!isEdit}
              />
            </Field>
            <Field label={isEdit ? "AWS secret access key (leave blank to keep current)" : "AWS secret access key"}>
              <input
                type="password"
                className="input"
                value={form.route53_secret_access_key}
                onChange={(e) => setForm({ ...form, route53_secret_access_key: e.target.value })}
                placeholder={isEdit ? "••••••••••••" : "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}
                required={!isEdit}
              />
            </Field>
          </>
        )}

        <Field label="Kubernetes namespace">
          <input
            className="input"
            value={form.k8s_namespace}
            onChange={(e) => setForm({ ...form, k8s_namespace: e.target.value })}
            placeholder="default"
            required
          />
        </Field>

        <Field label="Kubernetes secret name">
          <input
            className="input"
            value={form.k8s_secret_name}
            onChange={(e) => setForm({ ...form, k8s_secret_name: e.target.value })}
            placeholder="example-com-tls"
            required
          />
        </Field>

        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={form.auto_renew}
            onChange={(e) => setForm({ ...form, auto_renew: e.target.checked })}
          />
          Auto-renew
        </label>

        <Field label="Renew before expiry (days)">
          <input
            type="number"
            min={1}
            className="input"
            value={form.renew_before_days}
            onChange={(e) => setForm({ ...form, renew_before_days: Number(e.target.value) })}
          />
        </Field>

        <div className="flex gap-2">
          <button
            type="submit"
            disabled={mutation.isPending}
            className="rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-white dark:text-slate-900"
          >
            {mutation.isPending ? "Saving..." : "Save"}
          </button>
          <button
            type="button"
            onClick={() => navigate(-1)}
            className="rounded-md px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1 text-sm">
      <span className="font-medium">{label}</span>
      {children}
    </label>
  );
}
