export default function camera(camera, index) {
  return (
    <camera-feed key={camera.src} tabindex={index} reload="30000">
      <img
        src={`/image/${encodeURIComponent(camera.src)}`}
        alt={camera.alt}
        loading="lazy"
      />
      <h4>{camera.alt}</h4>
    </camera-feed>
  );
}
