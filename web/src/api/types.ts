export type ChallengeType = "http-01" | "dns-01";
export type DNSProvider = "" | "cloudflare" | "route53";
export type DomainStatus = "pending" | "active" | "failed" | "expired";
export type JobStatus = "running" | "success" | "failed";
export type TriggerType = "manual" | "scheduled";

export interface Domain {
  id: string;
  name: string;
  challenge_type: ChallengeType;
  dns_provider: DNSProvider;
  k8s_namespace: string;
  k8s_secret_name: string;
  auto_renew: boolean;
  renew_before_days: number;
  status: DomainStatus;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface DomainInput {
  name: string;
  challenge_type: ChallengeType;
  dns_provider: DNSProvider;
  k8s_namespace: string;
  k8s_secret_name: string;
  auto_renew: boolean;
  renew_before_days: number;
  cloudflare_api_token?: string;
  route53_access_key_id?: string;
  route53_secret_access_key?: string;
}

export interface DomainExportResponse {
  version: number;
  exported_at: string;
  domains: DomainInput[];
}

export type ImportAction = "created" | "updated" | "error";

export interface ImportResultItem {
  name: string;
  action: ImportAction;
  error?: string;
}

export interface ImportResponse {
  results: ImportResultItem[];
}

export interface Certificate {
  id: string;
  domain_id: string;
  issued_at: string;
  expires_at: string;
  status: string;
  created_at: string;
}

export interface CertificateDetail extends Certificate {
  serial_number?: string;
  issuer?: string;
  subject_cn?: string;
  dns_names?: string[];
}

export interface RenewalJob {
  id: string;
  domain_id: string;
  trigger: TriggerType;
  status: JobStatus;
  message?: string;
  started_at: string;
  finished_at?: string;
}
