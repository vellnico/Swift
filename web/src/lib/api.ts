/** Panel root (`/<panel_path>/`) derived from where index.html was served. */
export function panelBase(pathname: string): string {
  const parts = pathname.split('/').filter(Boolean);
  return parts.length ? `/${parts[0]}/` : '/';
}

export function apiURL(path: string, pathname = location.pathname): string {
  return panelBase(pathname) + 'api/' + path.replace(/^\/+/, '');
}
