export function shouldResyncForGenerationGap(previousGeneration: number, incomingGeneration: number) {
    if (previousGeneration <= 0) {
        return false;
    }

    return incomingGeneration > previousGeneration + 1;
}