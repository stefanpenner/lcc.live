import camera from "~/camera";
import { json } from "@remix-run/node"; 
import { useLoaderData } from "@remix-run/react";

export const loader = async () => {
  return json([
    {
      src: "https://udottraffic.utah.gov/1_devices/aux14605.jpeg",
      alt: "Wasatch Blvd @ BCC",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16212.jpeg",
      alt: "Dogwood",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16213.jpeg",
      alt: "S Curves",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16215.jpeg",
      alt: "Cardiff Fork",
    },
    {
      src: "https://udottraffic.utah.gov/1_devices/aux16216.jpeg",
      alt: "Silver Fork",
    },
    {
      src: "https://webcams.solitudemountain.com/LCMC.jpg",
      alt: "Last Chance",
    },
    {
      src: "https://webcams.solitudemountain.com/rh1.jpg",
      alt: "Roundhouse (1)",
    },
    {
      src: "https://webcams.solitudemountain.com/mbl.jpg",
      alt: "Moonbeam Village",
    },
    {
      src: "https://webcams.solitudemountain.com/rh2.jpg",
      alt: "Roundhouse (2)",
    },
    {
      src: "https://webcams.solitudemountain.com/ph.jpg",
      alt: "Powderhorn",
    },
  ]);
};

export default function BCC() {
  return <section id="container">
    {useLoaderData().map(camera)}
    </section>;
}
