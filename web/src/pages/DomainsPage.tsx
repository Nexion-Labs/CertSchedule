import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { Link } from "react-router-dom";
import { downloadCertbotArchive, exportDomains, importDomains, listDomains } from "../api/client";
import type { ImportResponse } from "../api/types";
import { StatusBadge } from "../components/StatusBadge";

export function DomainsPage() {
  const { data: domains, isLoading, error } = useQuery({ queryKey: ["domains"], queryFn: listDomains });
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [includeCredentials, setIncludeCredentials] = useState(false);
  const [importSummary, setImportSummary] = useState<ImportResponse | null>(null);
  const [importError, setImportError] = useState<string | null>(null);

  const handleExport = async () => {
    const data = await exportDomains(includeCredentials);
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `certschedule-domains-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const importMutation = useMutation({
    mutationFn: importDomains,
    onSuccess: (result) => {
      setImportSummary(result);
      setImportError(null);
      queryClient.invalidateQueries({ queryKey: ["domains"] });
    },
    onError: () => {
      setImportError("Import failed: the file may not be valid JSON in the expected format.");
      setImportSummary(null);
    },
  });

  const handleImportFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    try {
      const text = await file.text();
      const parsed = JSON.parse(text);
      const domainsToImport = Array.isArray(parsed) ? parsed : parsed.domains;
      if (!Array.isArray(domainsToImport)) {
        setImportError("Invalid file: expected an export file with a top-level \"domains\" array.");
        setImportSummary(null);
        return;
      }
      importMutation.mutate(domainsToImport);
    } catch {
      setImportError("Invalid file: could not parse JSON.");
      setImportSummary(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">Domains</h1>
        <div className="flex items-center gap-2">
          <label className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
            <input
              type="checkbox"
              checked={includeCredentials}
              onChange={(e) => setIncludeCredentials(e.target.checked)}
            />
            include credentials
          </label>
          <button
            type="button"
            onClick={handleExport}
            className="rounded-md border border-slate-300 px-3 py-1.5 text-sm font-medium hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-900"
          >
            Export
          </button>
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={importMutation.isPending}
            className="rounded-md border border-slate-300 px-3 py-1.5 text-sm font-medium hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:hover:bg-slate-900"
          >
            {importMutation.isPending ? "Importing..." : "Import"}
          </button>
          <input ref={fileInputRef} type="file" accept="application/json" onChange={handleImportFileChange} className="hidden" />
          <button
            type="button"
            onClick={() => {
              if (confirm("Download certbot's full data directory? It contains plaintext private keys and DNS credentials - treat the file as a secret.")) {
                downloadCertbotArchive();
              }
            }}
            className="rounded-md border border-red-300 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-50 dark:border-red-900 dark:text-red-400 dark:hover:bg-red-900/20"
          >
            Download certbot data
          </button>
          <Link
            to="/domains/new"
            className="rounded-md bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-700 dark:bg-white dark:text-slate-900"
          >
            + New domain
          </Link>
        </div>
      </div>

      {importError && <p className="text-sm text-red-600">{importError}</p>}
      {importSummary && (
        <div className="rounded-md border border-slate-200 px-3 py-2 text-sm dark:border-slate-800">
          Import finished:{" "}
          {["created", "updated", "error"].map((action) => {
            const count = importSummary.results.filter((r) => r.action === action).length;
            return count > 0 ? `${count} ${action}` : null;
          })
            .filter(Boolean)
            .join(", ") || "no changes"}
          .
          {importSummary.results.some((r) => r.action === "error") && (
            <ul className="mt-1 list-disc pl-5 text-red-600">
              {importSummary.results
                .filter((r) => r.action === "error")
                .map((r, i) => (
                  <li key={i}>
                    {r.name || "(missing name)"}: {r.error}
                  </li>
                ))}
            </ul>
          )}
        </div>
      )}

      {isLoading && <p className="text-sm text-slate-500">Loading...</p>}
      {error && <p className="text-sm text-red-600">Failed to load domains.</p>}

      {domains && domains.length === 0 && (
        <p className="text-sm text-slate-500">No domains yet. Create one to get started.</p>
      )}

      <div className="divide-y divide-slate-200 rounded-lg border border-slate-200 dark:divide-slate-800 dark:border-slate-800">
        {domains?.map((d) => (
          <Link
            key={d.id}
            to={`/domains/${d.id}`}
            className="flex items-center justify-between px-4 py-3 hover:bg-slate-50 dark:hover:bg-slate-900"
          >
            <div>
              <div className="font-medium">{d.name}</div>
              <div className="text-xs text-slate-500 dark:text-slate-400">
                {d.challenge_type}
                {d.dns_provider ? ` · ${d.dns_provider}` : ""} · {d.k8s_namespace}/{d.k8s_secret_name}
              </div>
            </div>
            <div className="flex items-center gap-2">
              {d.auto_renew && (
                <span className="text-xs text-slate-500 dark:text-slate-400">auto-renew ({d.renew_before_days}d)</span>
              )}
              <StatusBadge status={d.status} />
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
