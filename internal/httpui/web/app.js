"use strict";

const state = {
  cases: [],
  current: null,
  activeTab: "plan",
  timeline: [],
};

const statusLabels = {
  draft: "草稿",
  pending_evaluation: "待评估",
  remediation: "整改中",
  pending_review: "待复核",
  released: "已放行",
};

const eventLabels = {
  "case.created": "建立放行单",
  "plan.updated": "更新动作方案",
  "evaluation.requested": "提交规则评估",
  "evaluation.completed": "完成规则评估",
  "evidence.submitted": "提交整改证据",
  "review.requested": "申请安全复核",
  "finding.accepted": "接受风险证据",
  "finding.returned": "退回风险整改",
  "case.released": "签署安全放行",
};

const roleLabels = {
  technical_director: "剧场技术总监",
  mechanical_lead: "舞台机械主管",
  safety_reviewer: "演出安全复核员",
};

const el = (id) => document.getElementById(id);
const escapeHTML = (value) => String(value ?? "").replace(/[&<>'"]/g, (char) => ({"&":"&amp;","<":"&lt;",">":"&gt;","'":"&#39;",'"':"&quot;"})[char]);
const requestID = () => `web-${Date.now()}-${crypto.randomUUID()}`;
const formatTime = (value) => value ? new Intl.DateTimeFormat("zh-CN", {dateStyle: "medium", timeStyle: "short", hour12: false}).format(new Date(value)) : "-";
const formatSize = (size) => size < 1024 ? `${size} B` : size < 1048576 ? `${(size / 1024).toFixed(1)} KiB` : `${(size / 1048576).toFixed(1)} MiB`;

function actorHeaders(contentType = true) {
  const headers = {
    "X-Actor-Name": el("actor-name").value.trim(),
    "X-Actor-Role": el("actor-role").value,
  };
  if (contentType) headers["Content-Type"] = "application/json";
  return headers;
}

async function api(path, options = {}) {
  const response = await fetch(path, options);
  let payload;
  try {
    payload = await response.json();
  } catch (_) {
    throw new Error(`服务返回了无法解析的响应（${response.status}）`);
  }
  if (!response.ok) {
    const issue = payload.error || {};
    const fields = (issue.fields || []).map((item) => `${item.field}: ${item.message}`).join("；");
    throw new Error(fields || issue.message || `请求失败（${response.status}）`);
  }
  return payload.data;
}

function toast(message, error = false) {
  const item = document.createElement("div");
  item.className = `toast${error ? " error" : ""}`;
  item.textContent = message;
  el("toast-region").append(item);
  window.setTimeout(() => item.remove(), 4200);
}

async function loadHealth() {
  try {
    const health = await api("/api/health");
    el("service-state").textContent = `服务就绪 · ${health.rule_version}`;
  } catch (error) {
    el("service-state").textContent = "服务连接异常";
    toast(error.message, true);
  }
}

async function loadCases(selectID) {
  try {
    state.cases = await api("/api/cases");
    renderCaseList();
    if (selectID) await selectCase(selectID);
    else if (state.current) {
      const exists = state.cases.some((item) => item.id === state.current.id);
      if (!exists) clearCurrent();
    }
  } catch (error) {
    toast(error.message, true);
  }
}

function renderCaseList() {
  const query = el("case-search").value.trim().toLowerCase();
  const filtered = state.cases.filter((item) => [item.clearance_number, item.performance_name, item.venue_zone].some((value) => String(value).toLowerCase().includes(query)));
  el("case-count").textContent = `${state.cases.length} 项`;
  el("case-list").innerHTML = filtered.map((item) => `
    <button class="case-item${state.current?.id === item.id ? " active" : ""}" data-case-id="${escapeHTML(item.id)}" type="button">
      <span class="case-item-head"><strong>${escapeHTML(item.clearance_number)}</strong><span>r${item.revision}</span></span>
      <h3>${escapeHTML(item.performance_name)}</h3>
      <p>${escapeHTML(item.venue_zone)} · ${escapeHTML(statusLabels[item.status] || item.status)}</p>
    </button>`).join("") || `<div class="section-empty">没有匹配的放行单。</div>`;
  document.querySelectorAll("[data-case-id]").forEach((button) => button.addEventListener("click", () => selectCase(button.dataset.caseId)));
}

async function selectCase(caseID) {
  try {
    state.current = await api(`/api/cases/${encodeURIComponent(caseID)}`);
    el("empty-state").hidden = true;
    el("case-content").hidden = false;
    renderCaseList();
    renderCurrent();
    await loadTimeline();
  } catch (error) {
    toast(error.message, true);
  }
}

function clearCurrent() {
  state.current = null;
  el("empty-state").hidden = false;
  el("case-content").hidden = true;
  renderCaseList();
}

function renderCurrent() {
  const current = state.current;
  if (!current) return;
  el("case-number").textContent = current.clearance_number;
  el("case-status").textContent = statusLabels[current.status] || current.status;
  el("case-status").className = `status-badge status-${current.status}`;
  el("case-revision").textContent = `revision ${current.revision}`;
  el("performance-name").textContent = current.performance_name;
  el("case-meta").textContent = `${current.venue_zone} · ${formatTime(current.starts_at)} 至 ${formatTime(current.ends_at)}`;
  el("risk-count").textContent = current.findings.length;
  el("evaluate-case").hidden = current.status !== "draft";
  el("request-review").hidden = current.status !== "remediation";
  el("sign-case").hidden = current.status !== "pending_review";
  el("add-step").disabled = current.status !== "draft";
  el("save-plan").disabled = current.status !== "draft";
  renderPlan();
  renderRisks();
  renderReview();
  renderCertificate();
}

function renderPlan() {
  const rows = state.current.steps || [];
  el("plan-summary").textContent = `${rows.length} 个动作 · 按开始时间排序`;
  el("plan-rows").innerHTML = rows.map((step, index) => planRow(step, index)).join("") || planRow(newStep(1), 0);
  document.querySelectorAll(".remove-step").forEach((button) => button.addEventListener("click", () => {
    button.closest("tr").remove();
    renumberPlanRows();
  }));
}

function planRow(step, index) {
  const locked = state.current.status !== "draft" ? " disabled" : "";
  const interlocks = (step.interlock_codes || []).join(", ");
  return `<tr data-step-id="${escapeHTML(step.id || `step-${crypto.randomUUID()}`)}">
    <td><input class="step-sequence" type="number" min="1" value="${step.sequence || index + 1}"${locked}></td>
    <td><select class="step-device"${locked}>${["HOIST-A","HOIST-B","LIFT-1","REVOLVE-1","TRACK-1"].map((code) => `<option value="${code}"${code === step.device_code ? " selected" : ""}>${code}</option>`).join("")}</select></td>
    <td><select class="step-zone"${locked}>${["main","fly","pit","wing-left","wing-right"].map((zone) => `<option value="${zone}"${zone === step.zone ? " selected" : ""}>${zone}</option>`).join("")}</select></td>
    <td><input class="step-start" type="number" min="0" value="${step.starts_at_offset_ms ?? 0}"${locked}></td>
    <td><input class="step-duration" type="number" min="1" value="${step.duration_ms ?? 5000}"${locked}></td>
    <td><input class="step-load" type="number" min="0" step="0.1" value="${step.load_kg ?? 0}"${locked}></td>
    <td><label title="动作需要净空"><input class="step-requires" type="checkbox"${step.requires_clearance ? " checked" : ""}${locked}> 要求</label><label title="已确认净空"><input class="step-confirmed" type="checkbox"${step.clearance_confirmed ? " checked" : ""}${locked}> 已确认</label></td>
    <td><input class="step-interlocks" value="${escapeHTML(interlocks)}" placeholder="E-STOP, UPPER-LIMIT"${locked}></td>
    <td><button class="remove-step" type="button" aria-label="删除动作"${locked}>x</button></td>
  </tr>`;
}

function newStep(sequence) {
  return {id: `step-${crypto.randomUUID()}`, sequence, device_code: "HOIST-A", zone: "main", starts_at_offset_ms: (sequence - 1) * 6000, duration_ms: 5000, load_kg: 300, requires_clearance: true, clearance_confirmed: false, interlock_codes: ["E-STOP"]};
}

function collectSteps() {
  return [...el("plan-rows").querySelectorAll("tr")].map((row, index) => ({
    id: row.dataset.stepId,
    sequence: Number(row.querySelector(".step-sequence").value || index + 1),
    device_code: row.querySelector(".step-device").value,
    zone: row.querySelector(".step-zone").value,
    starts_at_offset_ms: Number(row.querySelector(".step-start").value),
    duration_ms: Number(row.querySelector(".step-duration").value),
    load_kg: Number(row.querySelector(".step-load").value),
    requires_clearance: row.querySelector(".step-requires").checked,
    clearance_confirmed: row.querySelector(".step-confirmed").checked,
    interlock_codes: row.querySelector(".step-interlocks").value.split(",").map((value) => value.trim()).filter(Boolean),
  }));
}

function renumberPlanRows() {
  [...el("plan-rows").querySelectorAll("tr")].forEach((row, index) => row.querySelector(".step-sequence").value = index + 1);
}

function renderRisks() {
  const findings = state.current.findings || [];
  const counts = {critical: 0, high: 0, evidence: 0, accepted: 0};
  findings.forEach((finding) => {
    if (finding.severity === "critical") counts.critical++;
    if (finding.severity === "high") counts.high++;
    if (finding.evidence) counts.evidence++;
    if (finding.status === "accepted") counts.accepted++;
  });
  el("risk-summary").innerHTML = [
    ["严重风险", counts.critical], ["高风险", counts.high], ["证据齐备", `${counts.evidence}/${findings.length}`], ["复核通过", `${counts.accepted}/${findings.length}`],
  ].map(([label, value]) => `<div class="metric"><span>${label}</span><strong>${value}</strong></div>`).join("");
  el("risk-list").innerHTML = findings.map((finding) => riskItem(finding)).join("") || `<div class="section-empty">当前方案尚未生成风险项。</div>`;
  document.querySelectorAll("[data-evidence-finding]").forEach((button) => button.addEventListener("click", () => openEvidence(button.dataset.evidenceFinding)));
}

function riskItem(finding) {
  const evidence = finding.evidence;
  const canSubmit = state.current.status === "remediation" && finding.status !== "accepted";
  return `<article class="risk-item" data-severity="${finding.severity}" data-status="${finding.status}">
    <div class="risk-item-head"><div><div class="risk-code"><code>${escapeHTML(finding.rule_code)}</code><span class="severity">${escapeHTML(finding.severity)}</span><span class="status-badge status-${finding.status}">${escapeHTML(findingStatus(finding.status))}</span></div><h4>${escapeHTML(finding.message)}</h4><p>${escapeHTML(finding.location)} · ${escapeHTML(finding.rule_version)}</p></div></div>
    <div class="risk-detail"><div class="evidence-meta">${evidence ? `<strong>${escapeHTML(evidence.original_name)} · ${formatSize(evidence.size_bytes)}</strong><span>${escapeHTML(evidence.note)} · SHA-256 ${escapeHTML(evidence.sha256.slice(0, 16))}...</span>` : `<span>尚未提交整改证据</span>`}</div>${canSubmit ? `<button class="secondary" data-evidence-finding="${escapeHTML(finding.id)}" type="button">${evidence ? "替换证据" : "提交证据"}</button>` : ""}</div>
  </article>`;
}

function findingStatus(status) {
  return ({open: "待整改", evidence_submitted: "证据已提交", accepted: "已接受", returned: "已退回"})[status] || status;
}

function renderReview() {
  const findings = state.current.findings || [];
  const accepted = findings.filter((item) => item.status === "accepted").length;
  el("review-progress").textContent = findings.length ? `${accepted}/${findings.length} 项已接受` : "当前方案无风险项";
  el("review-list").innerHTML = findings.map((finding) => reviewItem(finding)).join("") || `<div class="section-empty">暂无需要逐项复核的风险。</div>`;
  document.querySelectorAll("[data-review-action]").forEach((button) => button.addEventListener("click", () => reviewFinding(button.dataset.findingId, button.dataset.reviewAction === "accept")));
}

function reviewItem(finding) {
  const canReview = state.current.status === "pending_review" && finding.status !== "accepted";
  return `<article class="review-item" data-status="${finding.status}">
    <div class="review-item-head"><div><div class="risk-code"><code>${escapeHTML(finding.rule_code)}</code><span class="status-badge">${escapeHTML(findingStatus(finding.status))}</span></div><h4>${escapeHTML(finding.message)}</h4><p>${finding.evidence ? `${escapeHTML(finding.evidence.original_name)} · ${escapeHTML(finding.evidence.note)}` : "缺少证据"}</p>${finding.review_note ? `<p>复核意见：${escapeHTML(finding.review_note)}</p>` : ""}</div></div>
    ${canReview ? `<div class="review-controls"><input data-review-note="${escapeHTML(finding.id)}" placeholder="填写复核意见"><button class="secondary" data-review-action="return" data-finding-id="${escapeHTML(finding.id)}" type="button">退回整改</button><button class="primary" data-review-action="accept" data-finding-id="${escapeHTML(finding.id)}" type="button">接受证据</button></div>` : ""}
  </article>`;
}

async function loadTimeline() {
  if (!state.current) return;
  try {
    const view = await api(`/api/cases/${encodeURIComponent(state.current.id)}/timeline`);
    state.timeline = view.events || [];
    renderTimeline();
  } catch (error) {
    toast(error.message, true);
  }
}

function renderTimeline() {
  el("timeline-list").innerHTML = [...state.timeline].reverse().map((event) => `<li class="timeline-event"><span>revision ${event.revision}</span><div><strong>${escapeHTML(eventLabels[event.type] || event.type)}</strong><span>${escapeHTML(event.actor)} · ${escapeHTML(roleLabels[event.role] || event.role)}</span></div><span>${formatTime(event.occurred_at)}</span></li>`).join("") || `<li class="section-empty">暂无审计记录。</li>`;
}

function renderCertificate(certOverride) {
  const cert = certOverride || state.current.certificate;
  el("certificate-empty").hidden = Boolean(cert);
  el("certificate-view").hidden = !cert;
  if (!cert) return;
  el("cert-number").textContent = cert.clearance_number;
  el("cert-performance").textContent = cert.performance_name;
  el("cert-zone").textContent = cert.venue_zone;
  el("cert-rule").textContent = cert.rule_version;
  el("cert-signer").textContent = cert.signed_by;
  el("cert-time").textContent = formatTime(cert.signed_at);
  el("cert-digest").textContent = cert.plan_digest;
  el("cert-code").textContent = cert.verification_code;
}

function switchTab(name) {
  state.activeTab = name;
  document.querySelectorAll(".tab").forEach((tab) => tab.classList.toggle("active", tab.dataset.tab === name));
  document.querySelectorAll(".tab-panel").forEach((panel) => panel.classList.toggle("active", panel.id === `tab-${name}`));
}

async function createCase(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const command = {
    request_id: requestID(),
    performance_name: form.get("performance_name"),
    venue_zone: form.get("venue_zone"),
    starts_at: new Date(form.get("starts_at")).toISOString(),
    ends_at: new Date(form.get("ends_at")).toISOString(),
  };
  try {
    const created = await api("/api/cases", {method: "POST", headers: actorHeaders(), body: JSON.stringify(command)});
    el("case-dialog").close();
    event.currentTarget.reset();
    toast(`已建立 ${created.clearance_number}`);
    await loadCases(created.id);
  } catch (error) {
    toast(error.message, true);
  }
}

async function savePlan() {
  if (!state.current) return;
  try {
    state.current = await api(`/api/cases/${encodeURIComponent(state.current.id)}/plan`, {method: "PUT", headers: actorHeaders(), body: JSON.stringify({request_id: requestID(), expected_revision: state.current.revision, steps: collectSteps()})});
    toast("动作方案已保存");
    renderCurrent();
    await refreshAfterMutation();
  } catch (error) {
    toast(error.message, true);
  }
}

async function evaluateCase() {
  if (!state.current) return;
  try {
    state.current = await api(`/api/cases/${encodeURIComponent(state.current.id)}/evaluate`, {method: "POST", headers: actorHeaders(), body: JSON.stringify({request_id: requestID(), expected_revision: state.current.revision})});
    toast(`规则评估完成，生成 ${state.current.findings.length} 项风险`);
    switchTab("risks");
    renderCurrent();
    await refreshAfterMutation();
  } catch (error) {
    toast(error.message, true);
  }
}

function openEvidence(findingID) {
  const finding = state.current.findings.find((item) => item.id === findingID);
  if (!finding) return;
  const form = el("evidence-form");
  form.reset();
  form.elements.finding_id.value = findingID;
  form.elements.note.value = finding.evidence?.note || "";
  el("evidence-finding-code").textContent = `${finding.rule_code} · ${finding.location}`;
  el("evidence-file-meta").textContent = "";
  el("evidence-dialog").showModal();
}

async function evidenceChanged(event) {
  const file = event.target.files[0];
  if (!file) {
    el("evidence-file-meta").textContent = "";
    return;
  }
  const digest = await sha256Hex(await file.arrayBuffer());
  el("evidence-file-meta").textContent = `${file.name} · ${formatSize(file.size)} · SHA-256 ${digest.slice(0, 20)}...`;
}

async function submitEvidence(event) {
  event.preventDefault();
  const fields = new FormData(event.currentTarget);
  const file = fields.get("file");
  if (!(file instanceof File) || file.size === 0) {
    toast("请选择非空证据文件", true);
    return;
  }
  try {
    const digest = await sha256Hex(await file.arrayBuffer());
    const body = new FormData();
    body.set("request_id", requestID());
    body.set("expected_revision", String(state.current.revision));
    body.set("note", fields.get("note"));
    body.set("sha256", digest);
    body.set("file", file);
    const findingID = fields.get("finding_id");
    state.current = await api(`/api/cases/${encodeURIComponent(state.current.id)}/findings/${encodeURIComponent(findingID)}/evidence`, {method: "POST", headers: actorHeaders(false), body});
    el("evidence-dialog").close();
    toast("整改证据已校验并登记");
    renderCurrent();
    await refreshAfterMutation();
  } catch (error) {
    toast(error.message, true);
  }
}

async function requestReview() {
  try {
    state.current = await api(`/api/cases/${encodeURIComponent(state.current.id)}/review-request`, {method: "POST", headers: actorHeaders(), body: JSON.stringify({request_id: requestID(), expected_revision: state.current.revision})});
    toast("放行单已进入安全复核队列");
    switchTab("review");
    renderCurrent();
    await refreshAfterMutation();
  } catch (error) {
    toast(error.message, true);
  }
}

async function reviewFinding(findingID, accepted) {
  const input = document.querySelector(`[data-review-note="${CSS.escape(findingID)}"]`);
  const note = input?.value.trim() || "";
  if (!note) {
    toast("请填写复核意见", true);
    input?.focus();
    return;
  }
  try {
    state.current = await api(`/api/cases/${encodeURIComponent(state.current.id)}/findings/${encodeURIComponent(findingID)}/review`, {method: "POST", headers: actorHeaders(), body: JSON.stringify({request_id: requestID(), expected_revision: state.current.revision, accepted, note})});
    toast(accepted ? "证据已接受" : "风险项已退回整改");
    renderCurrent();
    await refreshAfterMutation();
  } catch (error) {
    toast(error.message, true);
  }
}

async function signCase() {
  try {
    const result = await api(`/api/cases/${encodeURIComponent(state.current.id)}/sign`, {method: "POST", headers: actorHeaders(), body: JSON.stringify({request_id: requestID(), expected_revision: state.current.revision})});
    toast("不可变放行凭证已签署");
    await selectCase(state.current.id);
    switchTab("certificate");
    renderCertificate(result.certificate);
  } catch (error) {
    toast(error.message, true);
  }
}

async function verifyCertificate(event) {
  event.preventDefault();
  const fields = new FormData(event.currentTarget);
  const query = new URLSearchParams({clearance_number: fields.get("clearance_number"), verification_code: fields.get("verification_code")});
  try {
    const result = await api(`/api/certificates/verify?${query}`);
    el("verify-dialog").close();
    const found = state.cases.find((item) => item.id === result.certificate.case_id);
    if (found) await selectCase(found.id);
    switchTab("certificate");
    renderCertificate(result.certificate);
    toast("凭证校验有效");
  } catch (error) {
    toast(error.message, true);
  }
}

async function sha256Hex(buffer) {
  const digest = await crypto.subtle.digest("SHA-256", buffer);
  return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, "0")).join("");
}

async function refreshAfterMutation() {
  await loadCases();
  await loadTimeline();
}

function setDefaultTimes() {
  const start = new Date(Date.now() + 86400000);
  start.setMinutes(0, 0, 0);
  const end = new Date(start.getTime() + 2 * 3600000);
  const localValue = (date) => new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
  const form = el("case-form");
  form.elements.starts_at.value = localValue(start);
  form.elements.ends_at.value = localValue(end);
}

function bindEvents() {
  el("new-case").addEventListener("click", () => { setDefaultTimes(); el("case-dialog").showModal(); });
  document.querySelectorAll(".close-dialog").forEach((button) => button.addEventListener("click", () => el("case-dialog").close()));
  document.querySelectorAll(".close-evidence").forEach((button) => button.addEventListener("click", () => el("evidence-dialog").close()));
  document.querySelectorAll(".close-verify").forEach((button) => button.addEventListener("click", () => el("verify-dialog").close()));
  el("case-form").addEventListener("submit", createCase);
  el("evidence-form").addEventListener("submit", submitEvidence);
  el("evidence-form").elements.file.addEventListener("change", evidenceChanged);
  el("verify-form").addEventListener("submit", verifyCertificate);
  el("case-search").addEventListener("input", renderCaseList);
  el("refresh-case").addEventListener("click", () => state.current && selectCase(state.current.id));
  el("add-step").addEventListener("click", () => { const count = el("plan-rows").children.length; el("plan-rows").insertAdjacentHTML("beforeend", planRow(newStep(count + 1), count)); const button = el("plan-rows").lastElementChild.querySelector(".remove-step"); button.addEventListener("click", () => { button.closest("tr").remove(); renumberPlanRows(); }); });
  el("save-plan").addEventListener("click", savePlan);
  el("evaluate-case").addEventListener("click", evaluateCase);
  el("request-review").addEventListener("click", requestReview);
  el("sign-case").addEventListener("click", signCase);
  el("open-verify").addEventListener("click", () => { const form = el("verify-form"); form.reset(); if (state.current?.certificate) { form.elements.clearance_number.value = state.current.certificate.clearance_number; form.elements.verification_code.value = state.current.certificate.verification_code; } el("verify-dialog").showModal(); });
  document.querySelectorAll(".tab").forEach((tab) => tab.addEventListener("click", () => switchTab(tab.dataset.tab)));
}

async function start() {
  bindEvents();
  await Promise.all([loadHealth(), loadCases()]);
}

start();
