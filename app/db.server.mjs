import App from "./app.server.mjs";

export async function camerasByCanyon(canyon = "lcc") {
  return App.app.db.cameras.findMany({
    where: { canyon },
  });
}

export async function hasHost(host) {
  return App.app.db.cameras.findFirst({
    where: { host },
  });
}
