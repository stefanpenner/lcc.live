import camera from "~/camera";

import { useLoaderData } from "@remix-run/react";
import { camerasByCanyon } from "~/db.server.mjs";

export const loader = async () => camerasByCanyon("lcc");

export default function Index() {
  return <section id="container">{useLoaderData().map(camera)}</section>;
}
