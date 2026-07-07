import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  deleteDomain,
  downloadCertificate,
  downloadCertificateKey,
  getCertificate,
  getDomain,
  issueCertificate,
  listCertificates,
  listJobs,
  renewCertificate,
} from "../api/client";
import { StatusBadge } from "../components/StatusBadge";
import { formatDateTime } from "../lib/formatDate";

export function DomainDetailPage() {
  const { id } = useParams();
  const domainId = id as string;
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data: d } = useQuery({ queryKey: ["domain", domainId], queryFn: () => getDomain(domainId) });
  const { data: certs } = useQuery({ queryKey: ["certificates", domainId], queryFn: () => listCertificates(domainId) });
  const { data: jobs } = useQuery({ queryKey: ["jobs", domainId], queryFn: () => listJobs(domainId) });

  const [viewingCertId, setViewingCertId] = useState<string | null>(null);
  const { data: certDetail } = useQuery({
    queryKey: ["certificate", domainId, viewingCertId],
    queryFn: () => getCertificate(domainId, viewingCertId as string),
    enabled: !!viewingCertId,
  });

  function invalidateAll() {
    queryClient.invalidateQueries({ queryKey: ["domain", domainId] });
    queryClient.invalidateQueries({ queryKey: ["certificates", domainId] });
    queryClient.invalidateQueries({ queryKey: ["jobs", domainId] });
  }

  const issueMutation = useMutation({ mutationFn: () => issueCertificate(domainId), onSuccess: invalidateAll });
  const renewMutation = useMutation({ mutationFn: () => renewCertificate(domainId), onSuccess: invalidateAll });
  const deleteMutation = useMutation({
    mutationFn: () => deleteDomain(domainId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["domains"] });
      navigate("/");
    },
  });

  if (!d) return <p className="text-sm text-slate-500">Loading...</p>;

  const hasCert = (certs?.length ?? 0) > 0;

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-xl font-semibold">{d.name}</h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            {d.challenge_type}
            {d.dns_provider ? ` · ${d.dns_provider}` : ""} · secret {d.k8s_namespace}/{d.k8s_secret_name}
          </p>
          {d.last_error && <p className="mt-1 text-sm text-red-600 dark:text-red-400">{d.last_error}</p>}
        </div>
        <StatusBadge status={d.status} />
      </div>

      <div className="flex flex-wrap gap-2">
        <button
          onClick={() => issueMutation.mutate()}
          disabled={issueMutation.isPending || hasCert}
          title={hasCert ? "Certificate already issued; use Renew instead" : ""}
          className="rounded-md bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-700 disabled:opacity-50 dark:bg-white dark:text-slate-900"
        >
          {issueMutation.isPending ? "Issuing..." : "Issue certificate"}
        </button>
        <button
          onClick={() => renewMutation.mutate()}
          disabled={renewMutation.isPending || !hasCert}
          className="rounded-md border border-slate-300 px-3 py-1.5 text-sm font-medium hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:hover:bg-slate-800"
        >
          {renewMutation.isPending ? "Renewing..." : "Renew now"}
        </button>
        <Link
          to={`/domains/${d.id}/edit`}
          className="rounded-md border border-slate-300 px-3 py-1.5 text-sm font-medium hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
        >
          Edit
        </Link>
        <button
          onClick={() => {
            if (confirm(`Delete domain ${d.name}? This does not remove the k8s secret.`)) {
              deleteMutation.mutate();
            }
          }}
          className="rounded-md border border-red-300 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-900/20"
        >
          Delete
        </button>
      </div>

      <section>
        <h2 className="mb-2 text-sm font-semibold text-slate-700 dark:text-slate-300">Certificate history</h2>
        <Table
          rows={certs}
          empty="No certificates issued yet."
          columns={["Status", "Issued", "Expires", "Actions"]}
          render={(c) => [
            <StatusBadge status={c.status} />,
            formatDateTime(c.issued_at),
            formatDateTime(c.expires_at),
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => setViewingCertId(c.id)}
                className="text-xs font-medium text-slate-600 hover:underline dark:text-slate-300"
              >
                View
              </button>
              <button
                type="button"
                onClick={() => downloadCertificate(domainId, c.id)}
                className="text-xs font-medium text-slate-600 hover:underline dark:text-slate-300"
              >
                Download
              </button>
            </div>,
          ]}
        />
      </section>

      {viewingCertId && (
        <div className="fixed inset-0 flex items-center justify-center bg-black/40 p-4" onClick={() => setViewingCertId(null)}>
          <div
            className="w-full max-w-lg space-y-3 rounded-lg border border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-950"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start justify-between">
              <h3 className="text-sm font-semibold">Certificate details</h3>
              <button
                type="button"
                onClick={() => setViewingCertId(null)}
                className="text-sm text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
              >
                ✕
              </button>
            </div>
            {!certDetail ? (
              <p className="text-sm text-slate-500">Loading...</p>
            ) : (
              <dl className="grid grid-cols-3 gap-x-2 gap-y-1 text-sm">
                <dt className="text-slate-500 dark:text-slate-400">Subject</dt>
                <dd className="col-span-2">{certDetail.subject_cn || "-"}</dd>
                <dt className="text-slate-500 dark:text-slate-400">Issuer</dt>
                <dd className="col-span-2">{certDetail.issuer || "-"}</dd>
                <dt className="text-slate-500 dark:text-slate-400">SANs</dt>
                <dd className="col-span-2">{certDetail.dns_names?.join(", ") || "-"}</dd>
                <dt className="text-slate-500 dark:text-slate-400">Serial</dt>
                <dd className="col-span-2 break-all font-mono text-xs">{certDetail.serial_number || "-"}</dd>
                <dt className="text-slate-500 dark:text-slate-400">Issued</dt>
                <dd className="col-span-2">{formatDateTime(certDetail.issued_at)}</dd>
                <dt className="text-slate-500 dark:text-slate-400">Expires</dt>
                <dd className="col-span-2">{formatDateTime(certDetail.expires_at)}</dd>
              </dl>
            )}
            <div className="flex flex-wrap gap-2 pt-2">
              <button
                type="button"
                onClick={() => downloadCertificate(domainId, viewingCertId)}
                className="rounded-md border border-slate-300 px-3 py-1.5 text-xs font-medium hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
              >
                Download fullchain.pem
              </button>
              <button
                type="button"
                onClick={() => {
                  if (confirm("Download the private key? Treat this file as a plaintext secret.")) {
                    downloadCertificateKey(domainId, viewingCertId);
                  }
                }}
                className="rounded-md border border-red-300 px-3 py-1.5 text-xs font-medium text-red-700 hover:bg-red-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-900/20"
              >
                Download private key
              </button>
            </div>
          </div>
        </div>
      )}

      <section>
        <h2 className="mb-2 text-sm font-semibold text-slate-700 dark:text-slate-300">Job log</h2>
        <Table
          rows={jobs}
          empty="No jobs run yet."
          columns={["Status", "Trigger", "Started", "Message"]}
          render={(j) => [
            <StatusBadge status={j.status} />,
            j.trigger,
            formatDateTime(j.started_at),
            j.message ?? "",
          ]}
        />
      </section>
    </div>
  );
}

function Table<T extends { id: string }>({
  rows,
  columns,
  render,
  empty,
}: {
  rows: T[] | undefined;
  columns: string[];
  render: (row: T) => React.ReactNode[];
  empty: string;
}) {
  if (!rows || rows.length === 0) {
    return <p className="text-sm text-slate-500">{empty}</p>;
  }
  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200 dark:border-slate-800">
      <table className="w-full text-left text-sm">
        <thead className="bg-slate-50 text-xs uppercase text-slate-500 dark:bg-slate-900 dark:text-slate-400">
          <tr>
            {columns.map((c) => (
              <th key={c} className="px-4 py-2 font-medium">
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
          {rows.map((row) => (
            <tr key={row.id}>
              {render(row).map((cell, i) => (
                <td key={i} className="px-4 py-2">
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
