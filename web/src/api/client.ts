import axios from "axios";
import type {
  Certificate,
  CertificateDetail,
  Domain,
  DomainExportResponse,
  DomainInput,
  ImportResponse,
  RenewalJob,
} from "./types";

const TOKEN_KEY = "certschedule_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

const client = axios.create({ baseURL: "/api/v1" });

client.interceptors.request.use((config) => {
  const token = getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      clearToken();
      if (window.location.pathname !== "/login") {
        window.location.href = "/login";
      }
    }
    return Promise.reject(error);
  }
);

export async function login(username: string, password: string): Promise<string> {
  const { data } = await client.post<{ token: string }>("/auth/login", { username, password });
  return data.token;
}

export async function listDomains(): Promise<Domain[]> {
  const { data } = await client.get<Domain[]>("/domains");
  return data;
}

export async function getDomain(id: string): Promise<Domain> {
  const { data } = await client.get<Domain>(`/domains/${id}`);
  return data;
}

export async function createDomain(input: DomainInput): Promise<Domain> {
  const { data } = await client.post<Domain>("/domains", input);
  return data;
}

export async function updateDomain(id: string, input: DomainInput): Promise<Domain> {
  const { data } = await client.put<Domain>(`/domains/${id}`, input);
  return data;
}

export async function deleteDomain(id: string): Promise<void> {
  await client.delete(`/domains/${id}`);
}

export async function issueCertificate(id: string): Promise<RenewalJob> {
  const { data } = await client.post<RenewalJob>(`/domains/${id}/issue`);
  return data;
}

export async function renewCertificate(id: string): Promise<RenewalJob> {
  const { data } = await client.post<RenewalJob>(`/domains/${id}/renew`);
  return data;
}

export async function listCertificates(id: string): Promise<Certificate[]> {
  const { data } = await client.get<Certificate[]>(`/domains/${id}/certificates`);
  return data;
}

export async function getCertificate(domainId: string, certId: string): Promise<CertificateDetail> {
  const { data } = await client.get<CertificateDetail>(`/domains/${domainId}/certificates/${certId}`);
  return data;
}

async function downloadPem(url: string, filename: string): Promise<void> {
  const { data } = await client.get<Blob>(url, { responseType: "blob" });
  const blobUrl = URL.createObjectURL(data);
  const a = document.createElement("a");
  a.href = blobUrl;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(blobUrl);
}

export async function downloadCertificate(domainId: string, certId: string): Promise<void> {
  await downloadPem(`/domains/${domainId}/certificates/${certId}/download`, "fullchain.pem");
}

export async function downloadCertificateKey(domainId: string, certId: string): Promise<void> {
  await downloadPem(`/domains/${domainId}/certificates/${certId}/download-key`, "privkey.pem");
}

export async function listJobs(id: string): Promise<RenewalJob[]> {
  const { data } = await client.get<RenewalJob[]>(`/domains/${id}/jobs`);
  return data;
}

export async function exportDomains(includeCredentials: boolean): Promise<DomainExportResponse> {
  const { data } = await client.get<DomainExportResponse>("/domains/export", {
    params: includeCredentials ? { include_credentials: "true" } : undefined,
  });
  return data;
}

export async function importDomains(domains: DomainInput[]): Promise<ImportResponse> {
  const { data } = await client.post<ImportResponse>("/domains/import", { domains });
  return data;
}

export async function downloadCertbotArchive(): Promise<void> {
  const { data } = await client.get<Blob>("/certbot/archive", { responseType: "blob" });
  const blobUrl = URL.createObjectURL(data);
  const a = document.createElement("a");
  a.href = blobUrl;
  a.download = `certbot-data-${new Date().toISOString().slice(0, 10)}.tar.gz`;
  a.click();
  URL.revokeObjectURL(blobUrl);
}
