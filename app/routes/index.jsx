import camera from "~/camera";

import { json } from "@remix-run/node"; 
import { useLoaderData } from "@remix-run/react";

export const loader = async () => {
  return json([
    {
      src: "/hdrelay/b9/8611e276-7ee5-42c0-b8cd-d9e1890e1cd4",
      alt: "Snowbird Snow Stake",
    },
    {
      src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/Collins_Snow_Stake.jpg",
      alt: "Alta Collins Snow Stake (12h)",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux14604.jpeg",
      alt: "Mouth of LCC",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16647.jpeg",
      alt: "Powerhouse",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16265.jpeg",
      alt: "Upper Vault",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16266.jpeg",
      alt: "Lisa Falls",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16268.jpeg",
      alt: "Tanners Flat",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16269.jpeg",
      alt: "White Pine",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16270.jpeg",
      alt: "White Pine Parking",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux17227.jpeg",
      alt: "Upper White Pine",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux17228.jpeg",
      alt: "Alta Bypass",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux17226.jpeg",
      alt: "Alta",
    },
    {
      src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/Superior.jpg",
      alt: "Mount Superior",
    },
    {
      src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/Highrustler.jpg",
      alt: "High Rustler",
    },
    {
      src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/sugar_peak.jpg",
      alt: "Sugarloaf Peak",
    },
    {
      src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/collins_dtc.jpg",
      alt: "Salt Lake Valley",
    },
    {
      src: "https://app.prismcam.com/public/helpers/realtime_preview.php?c=88&s=720",
      alt: "Mount Baldy",
    },
    {
      src: "https://backend.roundshot.com/cams/48fc223c0ed88474ecc2f884bf39de63/medium",
      alt: "Powder Paradise",
    },
    {
      src: "https://backend.roundshot.com/cams/44cfff4ff2a218a1178dbb105d95846a/medium",
      alt: "Hidden Peak",
    },
    {
      src: "/hdrelay/b9/db2a69c5-66e9-4c48-a713-919eaf191fc1",
      alt: "Snowbird Cam",
    },
    {
      src: "/hdrelay/b9/5780754f-8da1-4223-ab8a-6755d84cbc10",
      alt: "Mineral Basin",
    },
    {
      src: "hdrelay/b9/544432bd-3910-4888-aa9f-14b6f51a7eb5",
      alt: "Peruvian Gulch",
    },
    {
      src: "hdrelay/b7/61b2490be101c00b9c48374f",
      alt: "Tram Bullpen",
    },
  ]);
}

export default function Index() {
  return <section id="container">
    {useLoaderData().map(camera)}
  </section>;
}
