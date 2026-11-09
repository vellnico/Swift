import { mount } from 'svelte';
import Sub from './Sub.svelte';
import type { PageData } from './data';

const el = document.getElementById('swift-data');
const fallback: PageData = { lang: 'en', name: '', enabled: false, url: location.href.split('?')[0] ?? '', links: [], strings: {} };
const data: PageData = el?.textContent ? (JSON.parse(el.textContent) as PageData) : fallback;

const target = document.getElementById('app');
if (!target) throw new Error('missing #app');

export default mount(Sub, { target, props: { data } });
