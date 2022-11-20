import { hasHost } from "~/db.server.mjs";

export const loader = async (request) => {
  const url = new URL(decodeURIComponent(request.params.id));

  if (await hasHost(url.host)) {
    return fetch(url);
  } else {
    throw new Error(`Unknown Host: ${url.host}`);
  }
};
