package registry

import "llmplaceholder/internal/core/models"

var GlobalRegistry = []models.MockScenario{
	// ── Fintech / Billing ─────────────────────────────────────────────────────
	{
		ID:           "fintech_billing",
		Keywords:     []string{"invoice", "billing", "charge", "payment", "overdue", "revenue", "accounts receivable", "receipt"},
		FullResponse: "I pulled the billing ledger. The highest-priority overdue item is Invoice **#INV-2026-X5** from Acme Corp for **$1,200.00**, past due since May 1st. There is also an open support ticket TK-992 from billing@acmecorp.com flagging an overcharge dispute. Would you like me to draft a follow-up or flag this for collections?",
		MCPToolName:  "get_invoice_ledger",
		MCPToolData: map[string]interface{}{
			"status": "success",
			"data": []map[string]interface{}{
				{"id": "INV-2026-X5", "client": "Acme Corp", "amount": 1200.00, "status": "OVERDUE", "due_date": "2026-05-01"},
			},
		},
		StateKey: "recent_invoices",
	},

	// ── SaaS Growth / Analytics ───────────────────────────────────────────────
	{
		ID:           "saas_growth",
		Keywords:     []string{"churn", "mrr", "arr", "retention", "nps", "activation", "conversion", "cohort", "expansion", "ltv", "cac", "growth"},
		FullResponse: "Current MRR is **$128,400** (+6.2% MoM) with a churn rate of 2.1% and NRR of 118%. Three accounts churned this month totaling $51,600 ARR lost — the largest being Meridian Solutions ($24K) who switched to Tableau. Harlow Media Group is flagged at-risk with a health score of 38 and no login in 22 days. LTV:CAC ratio is healthy at 10.6x.",
		MCPToolName:  "get_growth_metrics",
		MCPToolData: map[string]interface{}{
			"mrr_usd": 128400, "churn_rate_pct": 2.1, "nrr_pct": 118,
		},
		StateKey: "growth_metrics",
	},
	{
		ID:           "saas_churn_accounts",
		Keywords:     []string{"churned", "lost accounts", "at risk", "health score", "churned customer"},
		FullResponse: "This month **3 accounts** churned totaling **$51,600 ARR**: Meridian Solutions ($24K, competitor switch), Dunlop & Associates ($18K, budget cut), Pinnacle Freight ($9.6K, low adoption). Currently **2 accounts are at risk**: Harlow Media Group (health 38, 22 days no login) and Crestwood Logistics (health 44, 3 open tickets). Recommend immediate CSM outreach.",
		MCPToolName:  "get_churn_report",
		MCPToolData: map[string]interface{}{
			"churned_mtd": 3, "arr_lost_usd": 51600,
		},
		StateKey: "top_churned_accounts",
	},
	{
		ID:           "saas_feature_adoption",
		Keywords:     []string{"feature adoption", "feature usage", "product adoption", "dau", "wau", "engagement"},
		FullResponse: "Top feature by adoption: **Data Export (CSV)** at 82% (1,390 WAU). **AI Summaries** follows at 71% and trending up. **Scheduled Reports** is declining at 29% — consider deprecation review. SSO/SAML is enabled by 61% of accounts but generates no weekly activity (config-and-forget pattern).",
		MCPToolName:  "get_feature_adoption",
		MCPToolData: map[string]interface{}{
			"top_feature": "Data Export (CSV)", "adoption_pct": 82,
		},
		StateKey: "feature_adoption",
	},

	// ── DevOps / SRE ──────────────────────────────────────────────────────────
	{
		ID:           "devops_incident",
		Keywords:     []string{"incident", "outage", "pager", "oncall", "down", "degraded", "sre", "p1", "p2", "error rate"},
		FullResponse: "Active incident **INC-2041 (P2)**: auth-service is reporting a 12.4% error rate on `/oauth/token` since 13:45 UTC, affecting ~320 users. Oncall is sarah.chen@nexus.io. Runbook: wiki.nexus.io/runbooks/auth-5xx. Previous incident INC-2040 (CDN cache miss, EU) was resolved at 11:58 UTC — root cause was a stale WAF rule config.",
		MCPToolName:  "get_incidents",
		MCPToolData: map[string]interface{}{
			"open_incidents": 1, "severity": "P2",
		},
		StateKey: "incidents",
	},
	{
		ID:           "devops_deployment",
		Keywords:     []string{"deploy", "deployment", "rollout", "release", "rollback", "canary", "version", "replicas", "service mesh"},
		FullResponse: "4 services tracked. **auth-service v1.9.3** is currently **degraded** with 12.4% error rate and p99 latency at 1840ms — recommend rollback to v1.9.2 or scale up replicas. api-gateway (v2.4.1) and payment-processor (v3.1.0) are healthy. A rollback decision on auth-service should go through INC-2041.",
		MCPToolName:  "get_deployment_status",
		MCPToolData: map[string]interface{}{
			"degraded_services": 1, "healthy_services": 3,
		},
		StateKey: "deployments",
	},
	{
		ID:           "devops_pipeline",
		Keywords:     []string{"pipeline", "ci", "cd", "build", "test", "github actions", "failed build", "test failure", "ci/cd"},
		FullResponse: "Latest pipeline run **run-9921** on `main` **FAILED** at the integration-tests stage after 312s. Failure: `TestOAuthFlowWithMFA` timed out after 30s — likely related to the degraded auth-service. Previous two runs on `main` and `feat/new-checkout` both succeeded. Recommend blocking the next `main` merge until INC-2041 is resolved.",
		MCPToolName:  "get_pipeline_runs",
		MCPToolData: map[string]interface{}{
			"last_run_status": "FAILED", "failed_stage": "integration-tests",
		},
		StateKey: "pipeline_runs",
	},

	// ── Homelab / Infrastructure ───────────────────────────────────────────────
	{
		ID:           "homelab_nodes",
		Keywords:     []string{"proxmox", "hypervisor", "node", "vm", "cpu", "ram", "server", "bare metal"},
		FullResponse: "Proxmox node **px-node-01** has been up 42 days with 18.4% CPU load and 32.1 GB RAM used of 64 GB total — headroom is healthy. ZFS pool **tank** is in a **DEGRADED** state: nvme1n1 shows UNAVAIL with 142 errors. Recommend replacing that drive before the pool loses redundancy. Pool **fast_cache** is ONLINE at 22% capacity.",
		MCPToolName:  "fetch_hypervisor_metrics",
		MCPToolData: map[string]interface{}{
			"node": "px-node-01", "cpu_load_pct": 18.4, "ram_used_gb": 32.1,
		},
		StateKey: "hypervisor_nodes",
	},
	{
		ID:           "homelab_storage",
		Keywords:     []string{"zfs", "pool", "disk", "storage", "nvme", "raid", "vdev", "capacity"},
		FullResponse: "ZFS pool **tank** is **DEGRADED** — nvme1n1 has UNAVAIL status with 142 I/O errors. This is a single-drive failure away from data loss. Recommend: (1) `zpool offline tank nvme1n1`, (2) replace drive, (3) `zpool replace tank nvme1n1`. Pool **fast_cache** is healthy at 22% capacity.",
		MCPToolName:  "get_zfs_pools",
		MCPToolData: map[string]interface{}{
			"degraded_pools": 1,
		},
		StateKey: "zfs_storage_pools",
	},
	{
		ID:           "homelab_containers",
		Keywords:     []string{"lxc", "container", "nextcloud", "jellyfin", "docker", "proxmox ct", "cloudflare tunnel"},
		FullResponse: "3 LXC containers: **nextcloud-app** (CT 100) and **jellyfin-media** (CT 101) are running. **cloudflare-tunnel** (CT 102) is **stopped** — this will break external access to all self-hosted services. Recommend `pct start 102` immediately.",
		MCPToolName:  "get_containers",
		MCPToolData: map[string]interface{}{
			"running": 2, "stopped": 1, "stopped_critical": "cloudflare-tunnel",
		},
		StateKey: "lxc_containers",
	},

	// ── Kernel / Low-level Debug ───────────────────────────────────────────────
	{
		ID:           "kernel_panic",
		Keywords:     []string{"kernel panic", "page fault", "general protection fault", "gdt", "idt", "interrupt", "apic", "segfault", "crash dump"},
		FullResponse: "Boot status is **kernel_panic**. The interrupt log shows a **General Protection Fault** (vector 13, error 0x0000) at 14:22:10 UTC followed immediately by a **Page Fault** (vector 14, CR2=0x00000000DEADBEEF) — a null-pointer dereference in a ring-0 context. GDT looks well-formed. APIC is in periodic timer mode. This pattern suggests a bad kernel pointer in an interrupt handler. Check recent driver patches.",
		MCPToolName:  "get_interrupt_logs",
		MCPToolData: map[string]interface{}{
			"boot_status": "kernel_panic", "last_fault": "Page Fault",
		},
		StateKey: "interrupt_logs",
	},
	{
		ID:           "kernel_gdt",
		Keywords:     []string{"gdt", "segment descriptor", "selector", "ring0", "ring3", "privilege level", "memory protection"},
		FullResponse: "GDT has 3 entries: NULL descriptor (0x00), CODE_RING0 at selector 0x08 (base 0x0, limit 0xFFFFF, flags 0xCF — 32-bit granularity), and DATA_RING0 at selector 0x10 (flags 0x8F). No user-space (ring-3) descriptors are defined yet. This is a minimal flat-model GDT suitable for a kernel stub but not yet ready for user processes.",
		MCPToolName:  "get_gdt_state",
		MCPToolData: map[string]interface{}{
			"entry_count": 3, "ring3_entries": 0,
		},
		StateKey: "gdt_entries",
	},

	// ── Legal / Matters ────────────────────────────────────────────────────────
	{
		ID:           "legal_matters",
		Keywords:     []string{"matter", "contract", "litigation", "deposition", "brief", "counsel", "due diligence", "m&a", "patent"},
		FullResponse: "4 active matters. **M-2026-0388** (Velocity Pharma Patent Licensing) is **AT RISK** — the license agreement draft is due June 8th with 89 of 120 budgeted hours consumed. Partner Marcus Crane is assigned. **M-2026-0441** (Orion M&A Due Diligence) is on track for June 15th delivery at 71% of budget hours. Recommend escalating the Velocity Pharma deadline immediately.",
		MCPToolName:  "get_legal_matters",
		MCPToolData: map[string]interface{}{
			"active_matters": 4, "at_risk": 1,
		},
		StateKey: "matters",
	},
	{
		ID:           "legal_billing",
		Keywords:     []string{"hours billed", "wip", "realization", "utilization", "accounts receivable", "collections", "retainer"},
		FullResponse: "Firm billing summary: **$187,400 WIP** unbilled, **$342,000** invoiced MTD, **$298,000** collected (87% realization). Outstanding AR sits at **$441,000**. Utilization rate is 74%. Consider following up on the $44K gap between invoiced and collected — Thornfield Capital and Beacon Real Estate have open balances.",
		MCPToolName:  "get_billing_summary",
		MCPToolData: map[string]interface{}{
			"wip_usd": 187400, "realization_pct": 87,
		},
		StateKey: "billing_summary",
	},

	// ── Retail / Inventory ─────────────────────────────────────────────────────
	{
		ID:           "retail_inventory",
		Keywords:     []string{"inventory", "stock", "sku", "reorder", "out of stock", "purchase order", "warehouse", "supplier"},
		FullResponse: "**4 SKUs are below reorder point.** Critical: **JKT-RAIN-M-BLK** (Rain Jacket M/Black) is fully out of stock — back-order was placed May 30th with 21-day lead time, so no restocking until ~June 20th. **OUT-BOOT-WRK-11** has 3 units vs reorder point of 12. Two open POs: PO-2026-1104 (Columbia, $28,400, arriving June 18) and PO-2026-1098 (Danner, $9,200, arriving June 10).",
		MCPToolName:  "get_inventory_alerts",
		MCPToolData: map[string]interface{}{
			"low_stock_count": 4, "out_of_stock_count": 1,
		},
		StateKey: "inventory",
	},
	{
		ID:           "retail_sales",
		Keywords:     []string{"sales", "pos", "transactions", "revenue today", "basket", "conversion", "foot traffic", "store performance"},
		FullResponse: "Today's POS: **$12,840 revenue** across **341 transactions** (avg basket $37.65). **Seattle-Pike** is the underperformer at -16% vs MTD target — construction on Pike St. is suppressing foot traffic. **Portland-Main** is +2.3% vs target. Top-selling SKU today is FLEECE-ZIP-M-NVY. Return rate is 5.3% (18 returns, $520 value).",
		MCPToolName:  "get_pos_summary",
		MCPToolData: map[string]interface{}{
			"revenue_usd": 12840, "transactions": 341,
		},
		StateKey: "pos_summary",
	},
	{
		ID:           "retail_stores",
		Keywords:     []string{"store", "location", "branch", "mtd target", "region performance"},
		FullResponse: "4 stores tracked MTD. **Portland-Main** (+2.3%) and **Denver-Union** (+1.5%) are above target. **Boise-Downtown** is -5.8%. **Seattle-Pike** is the outlier at -16% — construction impact is confirmed. Recommend a promotional push or temporary staff redeployment to Seattle.",
		MCPToolName:  "get_store_performance",
		MCPToolData: map[string]interface{}{
			"above_target": 2, "below_target": 2,
		},
		StateKey: "store_performance",
	},

	// ── Security / SOC ────────────────────────────────────────────────────────
	{
		ID:           "security_events",
		Keywords:     []string{"threat", "breach", "alert", "siem", "intrusion", "malware", "ransomware", "brute force", "ioc", "mitre"},
		FullResponse: "**CRITICAL active alert**: EVT-20260602-8810 — privilege escalation by `svc-etl-worker` on prod-etl-03 (`sudo bash -i`), under investigation by K. Patel (CSIRT-2026-019). Also active: brute-force against vpn.shieldlayer.io from 185.234.219.11 (RU, 1,247 attempts) — BLOCKED. Data exfil attempt by j.morrison (2.4 GB to Dropbox) was also blocked. Recommend isolating prod-etl-03 immediately.",
		MCPToolName:  "get_security_events",
		MCPToolData: map[string]interface{}{
			"critical_events": 1, "high_events": 3,
		},
		StateKey: "security_events",
	},
	{
		ID:           "security_vulnerabilities",
		Keywords:     []string{"vulnerability", "cve", "patch", "cvss", "scan", "remediation", "log4j", "openssl", "nginx"},
		FullResponse: "Vulnerability scan (412 assets, 2026-06-02 06:00 UTC): **3 critical CVEs** require immediate attention. **CVE-2025-44228** (log4j 2.14, CVSS 10.0) on prod-etl-03 — patch available, same host as active CSIRT-2026-019. **CVE-2026-1882** (nginx/1.24.0, CVSS 9.8) on prod-edge-01 — patch available. **CVE-2026-3041** (OpenSSH 8.6 on bastion-01, CVSS 9.1) — no patch yet, apply mitigation controls.",
		MCPToolName:  "get_vulnerability_report",
		MCPToolData: map[string]interface{}{
			"critical": 3, "high": 17, "patch_available": 2,
		},
		StateKey: "vulnerability_summary",
	},

	// ── Fallback ──────────────────────────────────────────────────────────────
	{
		ID:           "fallback",
		Keywords:     []string{},
		FullResponse: "I received your prompt, but no specific demo keywords were matched. Try terms like: invoice, churn, incident, deploy, inventory, matter, threat, kernel panic. I am running in local mock mode.",
		MCPToolName:  "unknown",
		MCPToolData:  map[string]string{"error": "Tool not found in mock registry."},
	},
}
