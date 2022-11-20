import camera from "~/camera";
import { useLoaderData } from "@remix-run/react";
import { camerasByCanyon } from "~/db.server.mjs";

export const loader = async () => camerasByCanyon("bcc");

export default function BCC() {
  return <section id="container">{useLoaderData().map(camera)}</section>;
}
