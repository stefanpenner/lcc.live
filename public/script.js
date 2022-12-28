class Overlay extends HTMLElement {
  constructor() {
    super();
    this.addEventListener("click", () => this.hide());
    this.timer = null;
  }

  empty() {
    clearTimeout(this.timer);
    [...this.childNodes].forEach((x) => x.remove());
  }

  hide() {
    this.style.display = "none";
    this.empty();
  }

  reload() {
    this.timer = setTimeout(() => {
      forceReload(this.querySelector("img"));
      this.timer = setTimeout(() => this.reload(), 30_000);
    }, 1_000);
  }

  show() {
    this.style.display = "block";
    this.reload();
  }

  cameraWasClicked(camera) {
    if (!camera.closest("body")) {
      return;
    }
    const cloned = camera.cloneNode(true);
    cloned.removeAttribute("tab-index");

    this.empty();
    this.appendChild(cloned);
    this.show();
  }
}

customElements.define("the-overlay", Overlay);
document.addEventListener("keyup", (e) => {
  switch (e.key) {
    case "Escape": {
      document.querySelector("the-overlay").hide();
      break;
    }
    case "Enter": {
      const { activeElement } = document;
      if (
        activeElement.closest("camera-feed") &&
        !activeElement.closest("the-overlay")
      ) {
        document.querySelector("the-overlay").cameraWasClicked(activeElement);
      }
      break;
    }
  }
});

const maxWidth = window.matchMedia("(max-width: 724px)");

maxWidth.addListener(
  (e) => e.matches && document.querySelector("the-overlay").hide()
);

function findCamera(target) {
  return target.closest("camera-feed");
}

document.body.addEventListener("click", (e) => {
  let camera;
  if (maxWidth.matches === false && (camera = findCamera(e.target))) {
    document.querySelector("the-overlay").cameraWasClicked(camera);
  }
});

for (const image of [...document.querySelectorAll("img")]) {
  if (!("src" in image.dataset)) {
    image.dataset.src = image.src;
  }
  image.onerror = function () {
    if (this.src.includes("/oops.png")) {
      return;
    }
    this.src = "/oops.png";
  };
}

function forceReload(image) {
  const original = image.dataset.src || image.src;
  const sep = origin.includes("?") ? "&" : "?";

  image.src = `${original}${sep}_x=${Date.now()}`;
}

const wait = async (time) =>
  new Promise((resolve) => setTimeout(resolve, time));

(async function reloadRoadStatus() {
  await wait(5_000);

  forceReload(document.querySelector("road-status"));

  reloadRoadStatus();
})();

document.addEventListener("visibilitychange", (event) => {
  if (event.target.visibilityState !== "visible") {
    return;
  }
  document.querySelectorAll("img").forEach((image) => forceReload(image));
});

(async function reload() {
  for (const camera of [...document.querySelectorAll("camera-feed")]) {
    const image = camera.querySelector("img");
    await wait(Math.random() * 10_000);
    forceReload(image);
  }
  reload();
})();
