const apiBase = "/api";
const recordTypes = [
  "A",
  "AAAA",
  "CNAME",
  "MX",
  "NS",
  "PTR",
  "SOA",
  "SRV",
  "TXT",
  "CAA",
  "NAPTR",
  "DNSKEY",
  "DS",
  "RRSIG",
  "NSEC",
  "NSEC3",
  "TLS",
];

const state = {
  zones: [],
  forwardZones: [],
  records: [],
  selectedZoneId: localStorage.getItem("selectedZoneId") || "",
  selectedForwardZoneId: localStorage.getItem("selectedForwardZoneId") || "",
  selectedRecordId: "",
  editingRecordId: "",
  loading: false,
};

const el = {
  apiStatus: document.getElementById("apiStatus"),
  zoneCount: document.getElementById("zoneCount"),
  recordCount: document.getElementById("recordCount"),
  forwardZoneCount: document.getElementById("forwardZoneCount"),
  zoneList: document.getElementById("zoneList"),
  forwardZoneList: document.getElementById("forwardZoneList"),
  recordList: document.getElementById("recordList"),
  activeZoneLabel: document.getElementById("activeZoneLabel"),
  activeForwardLabel: document.getElementById("activeForwardLabel"),
  toast: document.getElementById("toast"),
  refreshAll: document.getElementById("refreshAll"),
  zoneForm: document.getElementById("zoneForm"),
  zoneId: document.getElementById("zoneId"),
  zoneName: document.getElementById("zoneName"),
  zoneForwardZoneId: document.getElementById("zoneForwardZoneId"),
  zoneTTL: document.getElementById("zoneTTL"),
  zoneSubmit: document.getElementById("zoneSubmit"),
  zoneReset: document.getElementById("zoneReset"),
  recordForm: document.getElementById("recordForm"),
  recordId: document.getElementById("recordId"),
  recordName: document.getElementById("recordName"),
  recordType: document.getElementById("recordType"),
  recordValue: document.getElementById("recordValue"),
  recordTTL: document.getElementById("recordTTL"),
  recordPriority: document.getElementById("recordPriority"),
  recordSubmit: document.getElementById("recordSubmit"),
  recordReset: document.getElementById("recordReset"),
  forwardZoneForm: document.getElementById("forwardZoneForm"),
  forwardZoneId: document.getElementById("forwardZoneId"),
  forwardZoneName: document.getElementById("forwardZoneName"),
  forwardZoneAddresses: document.getElementById("forwardZoneAddresses"),
  forwardZoneSubmit: document.getElementById("forwardZoneSubmit"),
  forwardZoneReset: document.getElementById("forwardZoneReset"),
};

function setLoading(isLoading) {
  state.loading = isLoading;
  el.apiStatus.textContent = isLoading ? "Syncing" : "API ready";
}

function showToast(message, tone = "success") {
  el.toast.textContent = message;
  el.toast.className = `toast visible ${tone}`;
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => {
    el.toast.className = "toast";
  }, 2800);
}

function extractMessage(payload) {
  return (
    payload?.error?.message ||
    payload?.error?.Message ||
    payload?.message ||
    payload?.data ||
    "Request completed"
  );
}

async function api(path, options = {}) {
  const response = await fetch(`${apiBase}${path}`, {
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(options.headers || {}),
    },
    ...options,
  });

  let payload = null;
  const text = await response.text();
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch (error) {
      payload = { data: text };
    }
  }

  if (!response.ok) {
    throw new Error(extractMessage(payload) || response.statusText);
  }

  return payload?.data ?? payload;
}

function unwrapList(payload) {
  return payload?.data || [];
}

function findZone(id) {
  return state.zones.find((zone) => zone.id === id) || null;
}

function findForwardZone(id) {
  return state.forwardZones.find((zone) => zone.id === id) || null;
}

function zoneNameById(id) {
  return findForwardZone(id)?.name || (id ? id : "none");
}

function recordLabel(record) {
  const owner = record.name === "@" ? "@" : record.name;
  return `${owner} - ${record.type}`;
}

function setZoneSelection(id) {
  state.selectedZoneId = id || "";
  localStorage.setItem("selectedZoneId", state.selectedZoneId);
  state.selectedRecordId = "";
  renderZoneSelection();
  renderZones();
  renderRecordState();
}

function setForwardSelection(id) {
  state.selectedForwardZoneId = id || "";
  localStorage.setItem("selectedForwardZoneId", state.selectedForwardZoneId);
  renderForwardSelection();
  renderForwardZones();
}

function resetZoneForm() {
  el.zoneId.value = "";
  el.zoneName.value = "";
  el.zoneForwardZoneId.value = "";
  el.zoneTTL.value = "";
  el.zoneSubmit.textContent = "Create zone";
}

function resetRecordForm() {
  el.recordId.value = "";
  el.recordName.value = "";
  el.recordType.value = "A";
  el.recordValue.value = "";
  el.recordTTL.value = "";
  el.recordPriority.value = "";
  el.recordSubmit.textContent = "Create record";
  state.selectedRecordId = "";
  state.editingRecordId = "";
}

function resetForwardZoneForm() {
  el.forwardZoneId.value = "";
  el.forwardZoneName.value = "";
  el.forwardZoneAddresses.value = "";
  el.forwardZoneSubmit.textContent = "Create forward zone";
}

function renderForwardOptions() {
  const current = el.zoneForwardZoneId.value;
  el.zoneForwardZoneId.innerHTML = `
    <option value="">none</option>
    <option value="default">default</option>
  `;

  for (const zone of state.forwardZones) {
    const option = document.createElement("option");
    option.value = zone.id;
    option.textContent = `${zone.name} - ${zone.id}`;
    el.zoneForwardZoneId.appendChild(option);
  }

  if (current) {
    el.zoneForwardZoneId.value = current;
  }
}

function renderMetrics() {
  el.zoneCount.textContent = String(state.zones.length);
  el.forwardZoneCount.textContent = String(state.forwardZones.length);
  el.recordCount.textContent = String(state.records.length);
}

function renderZoneSelection() {
  const zone = findZone(state.selectedZoneId);
  if (!zone) {
    el.activeZoneLabel.innerHTML = "No zone selected";
    el.recordSubmit.disabled = true;
    el.recordList.innerHTML = `
      <div class="muted-state">
        Select a zone to load its records and enable record creation.
      </div>
    `;
    return;
  }

  el.activeZoneLabel.innerHTML = `<strong>${zone.name}</strong> <span>TTL ${zone.ttl}</span>`;
  el.recordSubmit.disabled = false;
}

function renderForwardSelection() {
  const zone = findForwardZone(state.selectedForwardZoneId);
  if (!zone) {
    el.activeForwardLabel.textContent = "None selected";
    return;
  }

  el.activeForwardLabel.innerHTML = `<strong>${zone.name}</strong> <span>${zone.addresses.length} addresses</span>`;
}

function renderZones() {
  renderForwardOptions();

  if (!state.zones.length) {
    el.zoneList.innerHTML = `
      <div class="muted-state">No zones yet. Create one to start mapping DNS data.</div>
    `;
    return;
  }

  el.zoneList.innerHTML = "";
  for (const zone of state.zones) {
    const card = document.createElement("article");
    card.className = `item ${zone.id === state.selectedZoneId ? "active" : ""}`;
    card.innerHTML = `
      <div class="item-head">
        <div>
          <h3>${escapeHtml(zone.name)}</h3>
          <p class="meta">TTL ${zone.ttl} - ${zone.record_count ?? 0} records</p>
        </div>
        <div class="chips">
          <span class="chip">${escapeHtml(zone.forward_zone || "default")}</span>
        </div>
      </div>
      <div class="actions">
        <button class="mini-button" data-action="select">Select</button>
        <button class="mini-button" data-action="edit">Edit</button>
        <button class="mini-button danger" data-action="delete">Delete</button>
      </div>
    `;

    card.addEventListener("click", () => {
      setZoneSelection(zone.id);
      loadRecords(zone.id);
    });

    card.querySelector('[data-action="select"]').addEventListener("click", (event) => {
      event.stopPropagation();
      setZoneSelection(zone.id);
      loadRecords(zone.id);
    });

    card.querySelector('[data-action="edit"]').addEventListener("click", (event) => {
      event.stopPropagation();
      editZone(zone);
    });

    card.querySelector('[data-action="delete"]').addEventListener("click", (event) => {
      event.stopPropagation();
      deleteZone(zone);
    });

    el.zoneList.appendChild(card);
  }
}

function renderForwardZones() {
  if (!state.forwardZones.length) {
    el.forwardZoneList.innerHTML = `
      <div class="muted-state">No forward zones yet. Add upstream resolvers for fallback lookups.</div>
    `;
    return;
  }

  el.forwardZoneList.innerHTML = "";
  for (const zone of state.forwardZones) {
    const card = document.createElement("article");
    card.className = `item ${zone.id === state.selectedForwardZoneId ? "active" : ""}`;
    const addressChips = zone.addresses
      .map((address) => `<span class="chip">${escapeHtml(address)}</span>`)
      .join("");

    card.innerHTML = `
      <div class="item-head">
        <div>
          <h3>${escapeHtml(zone.name)}</h3>
          <p class="meta">${zone.addresses.length} address${zone.addresses.length === 1 ? "" : "es"}</p>
        </div>
        <div class="chips">${addressChips || '<span class="chip muted">empty</span>'}</div>
      </div>
      <div class="actions">
        <button class="mini-button" data-action="select">Select</button>
        <button class="mini-button" data-action="edit">Edit</button>
        <button class="mini-button danger" data-action="delete">Delete</button>
      </div>
    `;

    card.addEventListener("click", () => {
      setForwardSelection(zone.id);
    });

    card.querySelector('[data-action="select"]').addEventListener("click", (event) => {
      event.stopPropagation();
      setForwardSelection(zone.id);
    });

    card.querySelector('[data-action="edit"]').addEventListener("click", (event) => {
      event.stopPropagation();
      editForwardZone(zone);
    });

    card.querySelector('[data-action="delete"]').addEventListener("click", (event) => {
      event.stopPropagation();
      deleteForwardZone(zone);
    });

    el.forwardZoneList.appendChild(card);
  }
}

function renderRecords() {
  const zone = findZone(state.selectedZoneId);
  if (!zone) {
    el.recordList.innerHTML = "";
    return;
  }

  if (!state.records.length) {
    el.recordList.innerHTML = `
      <div class="muted-state">This zone has no records yet.</div>
    `;
    renderMetrics();
    return;
  }

  el.recordList.innerHTML = "";
  for (const record of state.records) {
    const card = document.createElement("article");
    card.className = `item ${record.id === state.selectedRecordId ? "active" : ""}`;
    card.innerHTML = `
      <div class="item-head">
        <div>
          <h3>${escapeHtml(recordLabel(record))}</h3>
          <p class="meta">${escapeHtml(record.value)}</p>
        </div>
        <div class="chips">
          <span class="chip">${record.ttl || 300}s</span>
          ${record.priority ? `<span class="chip">${record.priority}</span>` : ""}
        </div>
      </div>
      <div class="actions">
        <button class="mini-button" data-action="edit">Edit</button>
        <button class="mini-button danger" data-action="delete">Delete</button>
      </div>
    `;

    card.addEventListener("click", () => editRecord(record));

    card.querySelector('[data-action="edit"]').addEventListener("click", (event) => {
      event.stopPropagation();
      editRecord(record);
    });

    card.querySelector('[data-action="delete"]').addEventListener("click", (event) => {
      event.stopPropagation();
      deleteRecord(record);
    });

    el.recordList.appendChild(card);
  }

  renderMetrics();
}

function renderRecordState() {
  const zone = findZone(state.selectedZoneId);
  el.recordSubmit.disabled = !zone;
  el.recordSubmit.textContent = el.recordId.value ? "Update record" : "Create record";
  renderMetrics();
  if (zone) {
    el.activeZoneLabel.innerHTML = `<strong>${zone.name}</strong> <span>TTL ${zone.ttl}</span>`;
  }
}

async function loadZones() {
  const payload = await api("/zones");
  state.zones = unwrapList(payload);

  if (!state.selectedZoneId && state.zones.length) {
    setZoneSelection(state.zones[0].id);
  }

  if (state.selectedZoneId && !findZone(state.selectedZoneId) && state.zones.length) {
    setZoneSelection(state.zones[0].id);
  }

  renderZones();
  renderMetrics();
  renderZoneSelection();
}

async function loadForwardZones() {
  const payload = await api("/forward-zones");
  state.forwardZones = unwrapList(payload);

  if (state.selectedForwardZoneId && !findForwardZone(state.selectedForwardZoneId)) {
    setForwardSelection("");
  }

  renderForwardZones();
  renderForwardOptions();
  renderForwardSelection();
  renderMetrics();
}

async function loadRecords(zoneId = state.selectedZoneId) {
  if (!zoneId) {
    state.records = [];
    renderRecords();
    return;
  }

  const payload = await api(`/zones/${encodeURIComponent(zoneId)}/records`);
  const list = unwrapList(payload);
  state.records = list;

  const selectedRecord = state.records.find((record) => record.id === state.selectedRecordId);
  if (!selectedRecord) {
    state.selectedRecordId = "";
    state.editingRecordId = "";
  }

  renderRecords();
  renderRecordState();
}

function editZone(zone) {
  el.zoneId.value = zone.id;
  el.zoneName.value = zone.name;
  el.zoneForwardZoneId.value = zone.forward_zone || "";
  el.zoneTTL.value = zone.ttl ?? "";
  el.zoneSubmit.textContent = "Update zone";
  window.scrollTo({ top: 0, behavior: "smooth" });
}

function editForwardZone(zone) {
  el.forwardZoneId.value = zone.id;
  el.forwardZoneName.value = zone.name;
  el.forwardZoneAddresses.value = zone.addresses.join("\n");
  el.forwardZoneSubmit.textContent = "Update forward zone";
  setForwardSelection(zone.id);
  window.scrollTo({ top: 0, behavior: "smooth" });
}

function editRecord(record) {
  const zone = findZone(state.selectedZoneId);
  if (!zone) {
    return;
  }

  el.recordId.value = record.id;
  el.recordName.value = record.name;
  el.recordType.value = record.type;
  el.recordValue.value = record.value;
  el.recordTTL.value = record.ttl || "";
  el.recordPriority.value = record.priority || "";
  el.recordSubmit.textContent = "Update record";
  state.selectedRecordId = record.id;
  state.editingRecordId = record.id;
  window.scrollTo({ top: 0, behavior: "smooth" });
  renderRecords();
}

async function deleteZone(zone) {
  if (!confirm(`Delete zone ${zone.name}? This also removes its records.`)) {
    return;
  }

  await api(`/zones/${encodeURIComponent(zone.id)}`, { method: "DELETE" });
  showToast(`Deleted zone ${zone.name}`);
  resetZoneForm();
  if (state.selectedZoneId === zone.id) {
    setZoneSelection("");
    state.records = [];
    renderRecords();
  }
  await refreshAll();
}

async function deleteForwardZone(zone) {
  if (!confirm(`Delete forward zone ${zone.name}?`)) {
    return;
  }

  await api(`/forward-zones/${encodeURIComponent(zone.id)}`, { method: "DELETE" });
  showToast(`Deleted forward zone ${zone.name}`);
  resetForwardZoneForm();
  if (state.selectedForwardZoneId === zone.id) {
    setForwardSelection("");
  }
  await refreshAll();
}

async function deleteRecord(record) {
  const zone = findZone(state.selectedZoneId);
  if (!zone) {
    return;
  }

  if (!confirm(`Delete record ${record.name} ${record.type}?`)) {
    return;
  }

  await api(`/zones/${encodeURIComponent(zone.id)}/records/${encodeURIComponent(record.id)}`, {
    method: "DELETE",
  });
  showToast(`Deleted record ${record.name} ${record.type}`);
  if (state.editingRecordId === record.id) {
    resetRecordForm();
  }
  state.selectedRecordId = state.editingRecordId === record.id ? "" : state.selectedRecordId;
  await loadRecords(zone.id);
}

async function submitZone(event) {
  event.preventDefault();
  const zoneId = el.zoneId.value.trim();
  const forwardZoneId = el.zoneForwardZoneId.value.trim();
  const body = {
    name: el.zoneName.value.trim(),
    forward_zone_id: forwardZoneId === "default" || forwardZoneId === "" ? null : forwardZoneId,
    ttl: el.zoneTTL.value === "" ? null : Number(el.zoneTTL.value),
  };

  const payload = await api(zoneId ? `/zones/${encodeURIComponent(zoneId)}` : "/zones", {
    method: "POST",
    body: JSON.stringify(body),
  });

  showToast(`Saved zone ${payload.name || body.name}`);
  resetZoneForm();
  await refreshAll();
}

async function submitForwardZone(event) {
  event.preventDefault();
  const forwardZoneId = el.forwardZoneId.value.trim();
  const addresses = el.forwardZoneAddresses.value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

  const payload = await api(forwardZoneId ? `/forward-zones/${encodeURIComponent(forwardZoneId)}` : "/forward-zones", {
    method: "POST",
    body: JSON.stringify({
      name: el.forwardZoneName.value.trim(),
      addresses,
    }),
  });

  showToast(`Saved forward zone ${payload.name || el.forwardZoneName.value.trim()}`);
  resetForwardZoneForm();
  await refreshAll();
}

async function submitRecord(event) {
  event.preventDefault();
  const zone = findZone(state.selectedZoneId);
  if (!zone) {
    showToast("Select a zone before creating records", "error");
    return;
  }

  const recordId = el.recordId.value.trim();
  const body = {
    name: el.recordName.value.trim(),
    type: el.recordType.value,
    value: el.recordValue.value.trim(),
    ttl: el.recordTTL.value === "" ? null : Number(el.recordTTL.value),
    priority: el.recordPriority.value === "" ? null : Number(el.recordPriority.value),
  };

  const payload = await api(
    recordId
      ? `/zones/${encodeURIComponent(zone.id)}/records/${encodeURIComponent(recordId)}`
      : `/zones/${encodeURIComponent(zone.id)}/records`,
    {
      method: "POST",
      body: JSON.stringify(body),
    },
  );

  showToast(`Saved record ${payload.name || body.name}`);
  resetRecordForm();
  await loadRecords(zone.id);
}

async function refreshAll() {
  setLoading(true);
  try {
    await loadForwardZones();
    await loadZones();

    if (state.selectedZoneId) {
      await loadRecords(state.selectedZoneId);
    } else {
      renderRecords();
    }
    renderZones();
    renderForwardZones();
    renderMetrics();
    showToast("Data refreshed");
  } catch (error) {
    console.error(error);
    showToast(error.message || "Something went wrong", "error");
    el.apiStatus.textContent = "Sync failed";
  } finally {
    setLoading(false);
  }
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function populateRecordTypes() {
  el.recordType.innerHTML = "";
  for (const type of recordTypes) {
    const option = document.createElement("option");
    option.value = type;
    option.textContent = type;
    el.recordType.appendChild(option);
  }
}

function wireEvents() {
  el.refreshAll.addEventListener("click", refreshAll);
  el.zoneForm.addEventListener("submit", (event) => {
    submitZone(event).catch((error) => showToast(error.message || "Failed to save zone", "error"));
  });
  el.forwardZoneForm.addEventListener("submit", (event) => {
    submitForwardZone(event).catch((error) => showToast(error.message || "Failed to save forward zone", "error"));
  });
  el.recordForm.addEventListener("submit", (event) => {
    submitRecord(event).catch((error) => showToast(error.message || "Failed to save record", "error"));
  });

  el.zoneReset.addEventListener("click", () => {
    resetZoneForm();
  });
  el.forwardZoneReset.addEventListener("click", () => {
    resetForwardZoneForm();
  });
  el.recordReset.addEventListener("click", () => {
    resetRecordForm();
  });
}

function init() {
  populateRecordTypes();
  wireEvents();
  resetRecordForm();
  refreshAll().catch((error) => showToast(error.message || "Failed to initialize", "error"));
}

init();
