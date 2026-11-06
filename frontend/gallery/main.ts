import { mount } from "svelte";
import "../src/style.less";
import GalleryApp from "../src/gallery/GalleryApp.svelte";

mount(GalleryApp, { target: document.getElementById("app")! });
