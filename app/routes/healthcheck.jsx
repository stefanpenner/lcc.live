export async function loader({ request }) {
  console.log("[healthcheck]", request.headers.get("user-agent"));
  const host =
    request.headers.get("X-Forwarded-Host") ?? request.headers.get("host");

  const url = new URL("/", `http://${host}`);
  const { ok } = await fetch(url.toString(), { method: "HEAD" });
  if (ok) {
    return new Response("OK");
  } else {
    console.log("healthcheck ❌", { error });
    return new Response("ERROR", { status: 500 });
  }
}
