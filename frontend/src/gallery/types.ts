// Mirrors internal/app/gallery_server.go (GalleryImage, GalleryInfo). The
// gallery service exists only in the server build, so there are no generated
// bindings for it: these types are written by hand.

export type GalleryFormat = "fits" | "raster";
export type GalleryView = "grid" | "atlas" | "slideshow";
export type GalleryTheme = "blue" | "red" | "grey";

export interface GalleryImage {
  /** Opaque id the picture is requested by (the preview route's `path`). */
  path: string;
  title: string;
  format: GalleryFormat;
  telescope: string;
  filter: string;
  /** "2006-01-02", or empty. */
  date: string;
  /** Stacked subs and one sub's seconds, when known. */
  subs: number;
  subExposure: number;
  /** Total integration in seconds, when known. */
  exposure: number;
  solved: boolean;
  ra: number;
  dec: number;
  /** Arcsec per pixel. */
  pixelScale: number;
  rotation: number;
}

export interface GalleryInfo {
  title: string;
  subtitle: string;
  about: string;
  theme: GalleryTheme;
  view: GalleryView;
  format: string;
  homeUrl: string;
  homeLabel: string;
  /** The format the page opens on: the configured one, or the exports when "auto". */
  defaultFormat: GalleryFormat;
  /** False until the server has read a library snapshot. */
  loaded: boolean;
  /** The pictures of the grid and the slideshow, in both formats. */
  images: GalleryImage[];
  /** The plate-solved pictures of the sky atlas, in both formats. */
  sky: GalleryImage[];
}
