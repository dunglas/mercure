import { test, expect, Page } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.goto('/ui');
});

test.describe('Receive update', () => {
  test('should receive update on a string topic', async ({ page }) => {
    
  });
});
