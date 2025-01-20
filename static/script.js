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
      img = this.querySelector("img");
      if (img) {
        forceReload(this.querySelector("img"));
        this.timer = setTimeout(() => this.reload(), 30_000);
      }
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

const ORIGINAL_SRC = new WeakMap();
function ensureOriginalCachedSrc(image) {
  if (image === null) { return }
  // ensure the original src is set
  const src = new URL(image.src);
  if (src.searchParams.has('_x')) {
    src.searchParams.delete('_x')
    image.src = src.toString();
  }

  if (!ORIGINAL_SRC.has(image) && !image.src.includes('/s/oops.png')) {
    ORIGINAL_SRC.set(image, image.src);
  }
}

for (const image of [...document.querySelectorAll("img")]) {
  ensureOriginalCachedSrc(image);

  image.onerror = function() {
    if (this.src.includes("/s/oops.png")) {
      return;
    }
    this.src = "/s/oops.png";
  };
}


function forceReload(image) {
  ensureOriginalCachedSrc(image);

  const original = ORIGINAL_SRC.get(image);
  const sep = origin.includes("?") ? "&" : "?";

  image.src = `${original}${sep}_x=${Date.now()}`;
}

const wait = async (time) =>
  new Promise((resolve) => setTimeout(resolve, time));

document.addEventListener("visibilitychange", (event) => {
  if (event.target.visibilityState !== "visible") {
    return;
  }
  document.querySelectorAll("img").forEach((image) => forceReload(image));
});

(async function reloadImages() {
  for (const image of [...document.querySelectorAll("img")]) {
    forceReload(image);
  }
  await wait(2000 + (Math.random() * 8000)); // Random wait between 2-10 seconds
  reloadImages();
  self.console && console.log("images reloaded")
})();

(async function reloadPage() {
  await wait(600_000 + (Math.random() * 1_200_000)); // Random wait between 5 - ~25 minutes
  self.location.reload()
})();
