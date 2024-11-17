export default function camera(camera, index) {

  if (camera.kind == "iframe") {
    return (
      <camera-feed key={camera.src} tabindex={index} >
        <iframe
          src={camera.src}
          title="YouTube video player"
          frameborder="0"
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen>
        </iframe>
        <h4>{camera.alt}</h4>
      </camera-feed >
    )
  } else {
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
}
