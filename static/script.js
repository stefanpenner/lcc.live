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
      const img = this.querySelector("img");
      if (img) {
        reloadImage(this.querySelector("img"));
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

// for (const image of [...document.querySelectorAll("img")]) {
//   image.onerror = function() {
//     if (this.src.includes("/s/oops.png")) {
//       return;
//     }
//     this.src = "/s/oops.png";
//   };
// }


function forceReload(image) {
  image.src = image.src
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
  // todo: timer, abort controller etc.
  self.console && console.time && console.time("load images")
  await Promise.allSettled([...document.querySelectorAll("img")].map(reloadImage))
  self.console && console.timeEnd && console.timeEnd("load images")
  await wait(2_000);
  reloadImages();
})();

(async function reloadPage() {
  await wait(600_000 + (Math.random() * 1_200_000)); // Random wait between 5 - ~25 minutes
  self.location.reload()
})();

async function reloadImage(img) {
  img.dataset.src = img.dataset.src || img.src
  const request  = await fetch(img.dataset.src, {
    mode: 'same-origin',
    cache: 'default'
  });

  if (request.status === 200) {
    etag = request.headers.get('etag')
    if (img.dataset.etag != etag) {
      img.dataset.etag = etag
      img.src = URL.createObjectURL(await request.blob());
    }
  }
}
