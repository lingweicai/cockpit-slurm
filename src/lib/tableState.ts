export function buildTableResetHandler(...steps: Array<() => void>) {
    return () => {
        for (const step of steps) {
            step();
        }
    };
}
