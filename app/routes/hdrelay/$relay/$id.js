export const loader = async (request) => {
  const RELAY = /^[a-z][0-9]$/i;
  const ID = /^[a-z0-9-]+$/i;

  const { relay, id } = request.params;

  if (RELAY.test(relay) === false) {
    throw new Error(`invalid RELAY: ${relay}`);
  }
  if (ID.test(id) === false) {
    throw new Error(`invalid ID: ${id}`);
  }

  const url = `https://${relay}.hdrelay.com/camera/${id}/snapshot`;

  // TODO: implement caching
  return fetch(url);
}
