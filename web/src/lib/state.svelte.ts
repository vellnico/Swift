export type XrayStatus = 'ok' | 'busy' | 'off';

/** Live state pushed by the server over SSE (wired up in later phases). */
export const live = $state<{ xray: XrayStatus; xrayVersion: string }>({ xray: 'off', xrayVersion: '' });
