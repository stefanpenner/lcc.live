export async function loader() {
  console.log("[healthcheck]");
  return new Response("OK");
}
