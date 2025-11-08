const list = document.querySelector("[data-canyon-list]");
const canyonCount = document.querySelector("[data-canyon-count]");
const cameraCount = document.querySelector("[data-camera-count]");
const statusText = document.querySelector("[data-status-text]");
const updatedAt = document.querySelector("[data-updated-at]");
const refreshButton = document.querySelector("[data-refresh]");

const REFRESH_INTERVAL = 15000;
let refreshTimer;
let currentController;

const formatTime = (date) =>
  date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });

const applyState = (state) => {
  list.dataset.state = state;
  if (state === "loading") {
    statusText.textContent = "Loading…";
  }
};

const setError = (message) => {
  statusText.textContent = "Error";
  list.dataset.state = "idle";
  list.innerHTML = `
    <div class="placeholder">
      <div class="spinner"></div>
      <p>${message}</p>
    </div>
  `;
};

const buildCamera = (camera) => {
  const wrapper = document.createElement("article");
  wrapper.className = "camera";

  const title = document.createElement("strong");
  title.textContent = camera.alt || "Untitled camera";
  wrapper.append(title);

  const src = document.createElement("span");
  src.textContent = camera.src;
  wrapper.append(src);

  const chip = document.createElement("span");
  chip.className = "chip";
  chip.dataset.kind = camera.kind || "image";
  chip.textContent = (camera.kind || "image").toUpperCase();
  wrapper.append(chip);

  return wrapper;
};

const paint = (data) => {
  if (!Array.isArray(data) || data.length === 0) {
    list.innerHTML = `
      <div class="placeholder">
        <div class="spinner"></div>
        <p>No canyon data found.</p>
      </div>
    `;
    return;
  }

  const fragment = document.createDocumentFragment();

  data.forEach((canyon) => {
    const card = document.createElement("article");
    card.className = "card";
    card.style.viewTransitionName = `canyon-${canyon.id}`;

    const header = document.createElement("div");
    header.className = "card-header";

    const title = document.createElement("h2");
    title.textContent = `${canyon.name} (${canyon.id})`;

    const badge = document.createElement("span");
    badge.className = "badge";
    badge.textContent = `${(canyon.cameras || []).length} Cameras`;

    header.append(title, badge);
    card.append(header);

    if (canyon.status && (canyon.status.src || canyon.status.alt)) {
      const statusBlock = document.createElement("p");
      statusBlock.className = "status-line";
      const alt = canyon.status.alt || "Status camera";
      statusBlock.innerHTML = `<strong>${alt}</strong> · <span>${canyon.status.src || "n/a"}</span>`;
      card.append(statusBlock);
    }

    const grid = document.createElement("div");
    grid.className = "camera-list";

    (canyon.cameras || []).forEach((camera) => {
      grid.append(buildCamera(camera));
    });

    card.append(grid);
    fragment.append(card);
  });

  list.replaceChildren(fragment);
};

const render = (data) => {
  const update = () => paint(data);

  if (document.startViewTransition) {
    document.startViewTransition(update);
  } else {
    update();
  }
};

const updateSummary = (data) => {
  const canyonTotal = Array.isArray(data) ? data.length : 0;
  const cameraTotal = Array.isArray(data)
    ? data.reduce((acc, canyon) => acc + (canyon.cameras ? canyon.cameras.length : 0), 0)
    : 0;

  canyonCount.textContent = canyonTotal.toString();
  cameraCount.textContent = cameraTotal.toString();
  updatedAt.textContent = formatTime(new Date());
  statusText.textContent = "Live";
};

const scheduleNext = () => {
  clearTimeout(refreshTimer);
  refreshTimer = setTimeout(() => refreshData(), REFRESH_INTERVAL);
};

const fetchCanyons = async (signal) => {
  const response = await fetch("/_/admin/api/canyons", { signal, cache: "no-store" });
  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || "Request failed");
  }
  const payload = await response.json();
  return payload.data || [];
};

const refreshData = async (manual = false) => {
  if (currentController) {
    currentController.abort();
  }

  const controller = new AbortController();
  currentController = controller;

  applyState("loading");

  try {
    const data = await fetchCanyons(controller.signal);
    render(data);
    updateSummary(data);
    list.dataset.state = "idle";
    scheduleNext();
  } catch (error) {
    if (error.name === "AbortError") {
      return;
    }
    console.error(error);
    setError("Unable to reach Neon. Retrying…");
    scheduleNext();
  }
};

refreshButton?.addEventListener("click", () => {
  refreshData(true);
});

document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    clearTimeout(refreshTimer);
    if (currentController) {
      currentController.abort();
    }
  } else {
    refreshData(true);
  }
});

refreshData(true);

