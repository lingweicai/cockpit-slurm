declare const cockpit: {
  channel(options: Record<string, unknown>): {
    close: () => void;
    send: (data: Uint8Array | ArrayBuffer | number[]) => void;
    onmessage: ((message: unknown) => void) | null;
    onerror?: ((error: unknown) => void) | null;
    onclose?: (() => void) | null;
  };
  gettext(value: string): string;
  format(value: string, ...args: unknown[]): string;
  file(path: string): {
    watch: (callback: (content?: string) => void) => void;
    close: () => void;
  };
};
