// Ignore releases younger than a week, like Dependabot's cooldown, to leave time for compromised ones to be pulled.
export const before = new Date(
  Date.now() - 7 * 24 * 60 * 60 * 1000,
).toISOString();
