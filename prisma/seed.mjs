import { PrismaClient } from "@prisma/client";
import crypto from "crypto";

const cameras = [
  {
    src: "https://udottraffic.utah.gov/1_devices/aux14605.jpeg",
    alt: "Wasatch Blvd @ BCC",
    canyon: "bcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16212.jpeg",
    alt: "Dogwood",
    canyon: "bcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16213.jpeg",
    alt: "S Curves",
    canyon: "bcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16215.jpeg",
    alt: "Cardiff Fork",
    canyon: "bcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16216.jpeg",
    alt: "Silver Fork",
    canyon: "bcc",
  },
  {
    src: "https://webcams.solitudemountain.com/rh1.jpg",
    alt: "Roundhouse (1)",
    canyon: "bcc",
  },
  {
    src: "https://webcams.solitudemountain.com/mbl.jpg",
    alt: "Moonbeam Village",
    canyon: "bcc",
  },
  {
    src: "https://webcams.solitudemountain.com/rh2.jpg",
    alt: "Sunshine Bowl",
    canyon: "bcc",
  },
  {
    src: "https://webcams.solitudemountain.com/rh2.jpg",
    alt: "Roundhouse (2)",
    canyon: "bcc",
  },
  {
    src: "https://webcams.solitudemountain.com/ph.jpg",
    alt: "Powderhorn",
    canyon: "bcc",
  },
  {
    src: "https://b9.hdrelay.com/camera/8611e276-7ee5-42c0-b8cd-d9e1890e1cd4/snapshot",
    alt: "Snowbird Snow Stake",
    canyon: "lcc",
  },
  {
    src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/Collins_Snow_Stake.jpg",
    alt: "Alta Collins Snow Stake (12h)",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux14604.jpeg",
    alt: "Mouth of LCC",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16647.jpeg",
    alt: "Powerhouse",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16265.jpeg",
    alt: "Upper Vault",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16266.jpeg",
    alt: "Lisa Falls",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16268.jpeg",
    alt: "Tanners Flat",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16269.jpeg",
    alt: "White Pine",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux16270.jpeg",
    alt: "White Pine Parking",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux17227.jpeg",
    alt: "Upper White Pine",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux17228.jpeg",
    alt: "Alta Bypass",
    canyon: "lcc",
  },
  {
    src: "https://udottraffic.utah.gov/1_devices/aux17226.jpeg",
    alt: "Alta",
    canyon: "lcc",
  },
  {
    src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/Superior.jpg",
    alt: "Mount Superior",
    canyon: "lcc",
  },
  {
    src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/Highrustler.jpg",
    alt: "High Rustler",
    canyon: "lcc",
  },
  {
    src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/sugar_peak.jpg",
    alt: "Sugarloaf Peak",
    canyon: "lcc",
  },
  {
    src: "https://altaskiarea.s3-us-west-2.amazonaws.com/mountain-cams/collins_dtc.jpg",
    alt: "Salt Lake Valley",
    canyon: "lcc",
  },
  {
    src: "https://app.prismcam.com/public/helpers/realtime_preview.php?c=88&s=720",
    alt: "Mount Baldy",
    canyon: "lcc",
  },
  {
    src: "https://backend.roundshot.com/cams/48fc223c0ed88474ecc2f884bf39de63/medium",
    alt: "Powder Paradise",
    canyon: "lcc",
  },
  {
    src: "https://backend.roundshot.com/cams/44cfff4ff2a218a1178dbb105d95846a/medium",
    alt: "Hidden Peak",
    canyon: "lcc",
  },
  {
    src: "https://b9.hdrelay.com/camera/db2a69c5-66e9-4c48-a713-919eaf191fc1/snapshot",
    alt: "Snowbird Cam",
    canyon: "lcc",
  },
  {
    src: "https://b9.hdrelay.com/camera/5780754f-8da1-4223-ab8a-6755d84cbc10/snapshot",
    alt: "Mineral Basin",
    canyon: "lcc",
  },
  {
    src: "https://b7.hdrelay.com/camera/61b2490be101c00b9c48374f/snapshot",
    alt: "Tram Bullpen",
    canyon: "lcc",
  },
];

async function seed(prisma) {
  for (const { src, canyon, alt } of cameras) {
    const id = `canyon:${canyon}|src:${src}`;
    const host = new URL(src).host.toLowerCase();
    await prisma.cameras.upsert({
      where: { id },
      update: {},
      create: {
        id,
        src,
        alt,
        canyon,
        host,
      },
    });
  }
}

(async function main() {
  const prisma = new PrismaClient();
  try {
    await seed(prisma);
  } finally {
    prisma.$disconnect();
  }
})();
