export default {
  multipass: true,
  plugins: [
    {
      name: "preset-default",
      params: {
        overrides: {
          // Externally referenced via <use href="file.svg#id"> from platformIcon.ts,
          // TitleBar, ModalShell, AppLockOverlay and the About modal.
          cleanupIds: false,
          // Dropping it breaks scaling when the sprite is sized by its <use> site.
          removeViewBox: false,
          // img/platform tiles are rasterised by oksvg (internal/winutil/svg_raster.go)
          // for shortcut .ico files. It draws elliptical arcs and quadratics wrong and
          // does not error, so the canvas fallback never kicks in -- keep cubics.
          convertPathData: { makeArcs: false, convertToQ: false },
        },
      },
    },
  ],
};
