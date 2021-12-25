// test helpers
function fragment(string) {
  const template = document.createElement("template");
  template.innerHTML = string;
  return template.content;
}

function firstChild(string) {
  return fragment(string).firstChild;
}

async function assert(assertion, _message) {
  const value = await assertion();
  const string = assertion.toString().replace("() => ", "");

  if (value) {
    console.log(`pass: ${string}`);
    return true;
  } else {
    throw new Error(`expected: 'true' but got: '${value}' for: ${string}`);
  }
}

class TestRunner {
  constructor() {
    this.tests = [];
    this.running = false;
  }

  test(name, execution) {
    const groupName = `test: ${name}`;

    this.tests.push({
      groupName,
      name,
      execution,
    });
  }

  async runTests() {
    try {
      this.running = true;
      while (this.tests.length !== 0) {
        const { groupName, execution } = this.tests.pop();
        console.group(groupName);
        await execution();
        console.groupEnd(groupName);
      }
    } finally {
      this.running = false;
    }
  }
}

const suite = new TestRunner();

function findCamera(target) {
  return target.closest("camera-feed");
}

suite.test("findCame", async function () {
  assert(() => findCamera(firstChild`<h1></h1>`) === null);
  assert(() => findCamera(firstChild`<camera-feed></camera-feed>`) === null);
  assert(
    () =>
      findCamera(
        firstChild`<camera-feed><div></div></camera-feed>`.querySelector("div")
      ).tagName === "CAMERA_FEED"
  );
  assert(
    () =>
      findCamera(
        firstChild`<parent><div></div><camera-feed><p></p></camera-feed></parent>`.querySelector(
          "div"
        )
      ) === null
  );
});

const IMAGE_SRCS = new WeakMap();
const RELOAD_IMAGE = Symbol("reload image");

HTMLImageElement.prototype[RELOAD_IMAGE] = function () {
  if (!IMAGE_SRCS.has(this)) {
    IMAGE_SRCS.set(this, this.src);
  }

  this.referrerPolicy = "no-referrer";
  this.src = `${IMAGE_SRCS.get(this)}?_=${Date.now()}`;
};

suite.test("image[RELOAD_IMAGE]", async function () {
  const image = new Image();
  image.referrerPolicy = "no-referrer";
  image.src = "foo";
  image[RELOAD_IMAGE]();

  assert(() => /=\d+$/.test(image.src));
  const src = image.src;
  await new Promise((resolve) => setTimeout(resolve, 10));
  image[RELOAD_IMAGE]();
  assert(() => /=\d+$/.test(image.src));
  assert(() => image.src !== src);
});

class Overlay extends HTMLElement {
  constructor() {
    super();
    this.addEventListener("click", () => this.hide());
  }

  empty() {
    [...this.childNodes].forEach((x) => x.remove());
  }

  hide() {
    this.style.display = "none";
    this.empty();
  }

  show() {
    this.style.display = "block";
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

class RoadStatus extends HTMLElement {
  constructor() {
    super();

    this._onScroll = (_) => {
      const { classList } = this;
      const { scrollTop } = this.ownerDocument.body.parentElement;

      if (scrollTop <= 5) {
        classList.remove("floater");
      } else {
        classList.add("floater");
      }
    };

    this.addEventListener(
      "click",
      () => (this.ownerDocument.documentElement.scrollTop = 0)
    );
  }

  reload() {
    this.querySelectorAll("img").forEach((img) => img[RELOAD_IMAGE]());
  }

  connectedCallback() {
    this.ownerDocument.addEventListener("scroll", this._onScroll, true);
    const again = () => {
      this._timer = setTimeout(() => {
        this.reload();
        again();
      }, Number(this.getAttribute("reload")));
    };

    again();
  }

  disconnectedCallback() {
    this.ownerDocument.removeEventListener("scroll", this._onScroll);
    clearTimeout(this._timer);
  }
}

class CameraFeed extends HTMLElement {
  constructor() {
    super();
    this._timer = null;
  }

  attributeChangedCallback() {
    this.connectedCallback();
  }

  disconnectedCallback() {
    clearTimeout(this._timer);
  }

  reload() {
    this.querySelectorAll("img").forEach((img) => img[RELOAD_IMAGE]());
  }

  connectedCallback() {
    this.disconnectedCallback();

    const again = () => {
      this._timer = setTimeout(() => {
        this.reload();
        again();
      }, Number(this.getAttribute("reload")));
    };

    again();
  }
}

(async function main() {
  customElements.define("camera-feed", CameraFeed);
  customElements.define("road-status", RoadStatus);
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

  const maxWidth = window.matchMedia("(max-width: 667px)");

  maxWidth.addListener(
    (e) => e.matches && document.querySelector("the-overlay").hide()
  );

  document.body.addEventListener("click", (e) => {
    let camera;
    if (maxWidth.matches === false && (camera = findCamera(e.target))) {
      document.querySelector("the-overlay").cameraWasClicked(camera);
    }
  });

  document.addEventListener("visibilitychange", () => {
    document
      .querySelectorAll("camera-feed,road-status")
      .forEach((element) => element.reload());
  });

  // await suite.runTests();
})();
