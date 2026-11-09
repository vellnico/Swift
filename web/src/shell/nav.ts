import type { Component } from 'svelte';
import LayoutDashboard from '@lucide/svelte/icons/layout-dashboard';
import Layers from '@lucide/svelte/icons/layers';
import Users from '@lucide/svelte/icons/users';
import Route from '@lucide/svelte/icons/route';
import Settings from '@lucide/svelte/icons/settings';
import ScrollText from '@lucide/svelte/icons/scroll-text';
import Ellipsis from '@lucide/svelte/icons/ellipsis';
import type { RouteName } from '../lib/router.svelte';

export interface NavItem {
  route: RouteName;
  path: string;
  label: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  icon: Component<any>;
}

export const sidebarItems: NavItem[] = [
  { route: 'dashboard', path: '/', label: 'nav.dashboard', icon: LayoutDashboard },
  { route: 'profiles', path: '/profiles', label: 'nav.profiles', icon: Layers },
  { route: 'users', path: '/users', label: 'nav.users', icon: Users },
  { route: 'routing', path: '/routing', label: 'nav.routing', icon: Route },
  { route: 'settings', path: '/settings', label: 'nav.settings', icon: Settings },
  { route: 'log', path: '/log', label: 'nav.log', icon: ScrollText },
];

export const bottomItems: NavItem[] = [
  sidebarItems[0]!,
  sidebarItems[1]!,
  sidebarItems[2]!,
  { route: 'more', path: '/more', label: 'nav.more', icon: Ellipsis },
];

/** Sections reachable from the "More" tab on phones. */
export const moreRoutes: RouteName[] = ['routing', 'settings', 'log', 'more'];
