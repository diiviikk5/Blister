import { mount } from "svelte";
import "./styles/iso-brutal.css";
import "./styles/app.css";
import "./styles/desktop.css";
import { applyRice, defaultRice } from "./lib/rice";
import App from "./App.svelte";

applyRice(defaultRice);
export default mount(App, { target: document.getElementById("app")! });
