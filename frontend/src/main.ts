import { mount } from "svelte";
import "./style.less";
import App from "./App.svelte";
import { installContextMenuGuard } from "./lib/contextMenuGuard";

installContextMenuGuard();

const app = mount(App, {
  target: document.getElementById("app")!,
});

export default app;
