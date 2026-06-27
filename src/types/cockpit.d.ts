declare module 'cockpit' {
  export interface CockpitUserInfo {
    home?: string;
    uid?: number;
    name?: string;
  }

  export interface CockpitInfo {
    user?: CockpitUserInfo;
  }

  export interface CockpitChannelLike {
    send(message: string): void;
    close(): void;
    ready?: boolean;
    valid?: boolean;
    id?: string;
    options?: {
      spawn?: string[];
      [key: string]: unknown;
    };
    addEventListener?(event: string, listener: (event: unknown) => void): void;
    removeEventListener?(event: string, listener: (event: unknown) => void): void;
    on?(event: string, listener: (...args: unknown[]) => void): void;
    off?(event: string, listener: (...args: unknown[]) => void): void;
    addListener?(event: string, listener: (...args: unknown[]) => void): void;
    removeListener?(event: string, listener: (...args: unknown[]) => void): void;
  }

  export interface CockpitModule {
    gettext(message: string): string;
    format(message: string, ...values: Array<string | number>): string;
    channel(options: string | Record<string, unknown>): CockpitChannelLike;
    info?: CockpitInfo;
  }

  const cockpit: CockpitModule;
  export default cockpit;
}
