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
        },
      },
    },
  ],
};
