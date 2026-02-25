/**
 * @file Mercure Hub - Debugging Tools
 * @description This script powers the Mercure Hub debugging interface, handling SSE connections,
 * API interactions, and dynamic UI updates.
 */

// Note: This app requires internet access to load the SSE library from CDN.
// For offline/air-gapped environments, vendor this library locally.
import { fetchEventSource } from 'https://cdn.jsdelivr.net/npm/@microsoft/fetch-event-source/+esm';

/**
 * Custom error class to indicate an error is retriable.
 * @extends Error
 */
class RetriableError extends Error {}

/**
 * Custom error class to indicate an error is fatal and should not be retried.
 * @extends Error
 */
class FatalError extends Error {}

/**
 * Log level for the debugging tools UI.
 * Set to DEBUG by default since this is a developer tool where verbose logging is expected.
 * @type {'DEBUG'|'INFO'|'WARN'|'ERROR'}
 */
const LOG_LEVEL = 'DEBUG';

/**
 * A structured logging utility for consistent and readable console output.
 * Available at module scope for use by all functions in this file.
 */
const Logger = (() => {
  const levels = { DEBUG: 1, INFO: 2, WARN: 3, ERROR: 4 };
  const currentLevel = levels[LOG_LEVEL] || levels.INFO;
  const levelColors = {
    DEBUG: 'gray',
    INFO: '#3e8ed0',
    WARN: '#ffdd57',
    ERROR: '#f14668',
  };
  const levelTextColors = { WARN: 'black', DEBUG: 'white' };

  function log(level, message, ...args) {
    if (levels[level] < currentLevel) return;
    const timestamp = new Date().toISOString();
    const color = levelColors[level] || 'black';
    const textColor = levelTextColors[level] || 'white';
    const prefix = `%c[${timestamp}] [${level}]`;
    const style = `background: ${color}; color: ${textColor}; padding: 2px 4px; border-radius: 3px;`;
    const logMethod = console[level.toLowerCase()] || console.log;

    if (args.length > 0) {
      logMethod(prefix, style, message);
      console.groupCollapsed('%cDetails', 'color: gray; font-style: italic;');
      for (const arg of args) {
        console.dir(arg);
      }
      console.groupEnd();
    } else {
      logMethod(prefix, style, message);
    }
  }

  return {
    debug: (message, ...args) => log('DEBUG', message, ...args),
    info: (message, ...args) => log('INFO', message, ...args),
    warn: (message, ...args) => log('WARN', message, ...args),
    error: (message, ...args) => log('ERROR', message, ...args),
  };
})();

/**
 * Theme toggle functionality - runs immediately to prevent flash of wrong theme.
 * Cycles through: auto (system) → light → dark → auto
 */
(function initTheme() {
  const THEME_KEY = 'mercure-theme';
  const themes = ['auto', 'light', 'dark'];
  const icons = { auto: '◐', light: '☀', dark: '☾' };
  const titles = {
    auto: 'Theme: Auto (system)',
    light: 'Theme: Light',
    dark: 'Theme: Dark',
  };

  function getStoredTheme() {
    try {
      return localStorage.getItem(THEME_KEY) || 'auto';
    } catch (e) {
      Logger.debug('[Theme] localStorage unavailable, using default.', e);
      return 'auto';
    }
  }

  function setStoredTheme(theme) {
    try {
      localStorage.setItem(THEME_KEY, theme);
    } catch (e) {
      Logger.debug("[Theme] localStorage unavailable, theme won't persist.", e);
    }
  }

  function applyTheme(theme) {
    const html = document.documentElement;
    if (theme === 'auto') {
      html.removeAttribute('data-theme');
    } else {
      html.setAttribute('data-theme', theme);
    }
    updateHljsTheme(theme);
  }

  function updateHljsTheme(theme) {
    const hljsLink = document.getElementById('hljs-theme');
    if (!hljsLink) return;

    const baseUrl = 'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/styles/';
    let isDark = theme === 'dark';
    if (theme === 'auto') {
      isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    }
    hljsLink.href = isDark ? `${baseUrl}github-dark.min.css` : `${baseUrl}github.min.css`;
  }

  function updateToggleButton(theme) {
    const icon = document.getElementById('theme-icon');
    const button = document.getElementById('theme-toggle');
    if (icon) icon.textContent = icons[theme];
    if (button) button.dataset.tooltip = titles[theme];
  }

  function cycleTheme() {
    const current = getStoredTheme();
    const nextIndex = (themes.indexOf(current) + 1) % themes.length;
    const next = themes[nextIndex];
    setStoredTheme(next);
    updateToggleButton(next);
    if (document.startViewTransition) {
      document.startViewTransition(() => applyTheme(next));
    } else {
      applyTheme(next);
    }
  }

  // Apply stored theme immediately (before DOMContentLoaded) to prevent flash
  applyTheme(getStoredTheme());

  // Set up the toggle button once DOM is ready
  document.addEventListener('DOMContentLoaded', () => {
    const theme = getStoredTheme();
    updateToggleButton(theme);
    const button = document.getElementById('theme-toggle');
    if (button) button.addEventListener('click', cycleTheme);
  });
})();

/** Main application logic. */
(() => {
  /** Configuration constants for default values and application settings. */
  const defaultHubUrl = `${window.location.origin}/.well-known/mercure`;
  const CONFIG = {
    // Embedded fixture files served from public/fixtures/ via Go's embed.FS.
    UI_JWKS_PATH: './fixtures/jwks.json',
    UI_PUBLIC_PEM_PATH: './fixtures/public-key.pem',
    UI_PUBLIC_JWK_PATH: './fixtures/public-jwk.json',
    UI_PRIVATE_JWK_PATH: './fixtures/private-jwk.json',
    UI_DEMO_TOKEN_PATH: './fixtures/tokens.json',
    UI_DEMO_TOKEN_KEY: 'rs256',
    UI_HS256_TOKEN_KEY: 'hs256',
    UI_HS256_SECRET: '!ChangeThisMercureHubJWTSecretKey!',
    UI_DEFAULT_HUB_URL: defaultHubUrl,
    UI_PLACEHOLDER_TOPIC: `${defaultHubUrl}/ui/demo/books/1.jsonld`,
    // Controls whether the subscription monitoring SSE stays open when the tab is hidden
    FES_OPEN_WHEN_HIDDEN_PRESENCE: true, // Stay alive to track all state changes
    FES_RETRY_BASE_DELAY_MS: 1000,
    FES_RETRY_MAX_DELAY_MS: 30000,
    FES_RETRY_SUCCESS_RESET_MS: 60000,
  };

  /**
   * Cached demo tokens (RS256 and HS256) from embedded fixtures.
   * Populated once on init to avoid redundant network requests.
   */
  let cachedDemoTokens = null;

  /**
   * Fetches and caches demo tokens from embedded fixtures.
   * Returns cached tokens if already fetched.
   * @returns {Promise<{rs256: string, hs256: string}|null>}
   */
  async function fetchDemoTokens() {
    if (cachedDemoTokens) return cachedDemoTokens;

    try {
      const resp = await fetch(CONFIG.UI_DEMO_TOKEN_PATH);
      if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
      cachedDemoTokens = await resp.json();
      Logger.debug('[Init] Cached demo tokens (RS256 and HS256).');
      return cachedDemoTokens;
    } catch (err) {
      Logger.warn('[Init] Failed to fetch demo tokens.', err);
      showNotification(
        'Could not load demo tokens. Enter a JWT manually or check network connectivity.',
        'warning',
        6000,
      );
      return null;
    }
  }

  /**
   * Decodes a Base64URL-encoded string with proper UTF-8 support.
   * Wraps atob() with TextDecoder to correctly handle multi-byte UTF-8 characters.
   * @param {string} str - The Base64URL-encoded string to decode.
   * @returns {string} The decoded UTF-8 string.
   */
  function decodeBase64Url(str) {
    // Convert Base64URL to standard Base64
    const base64 = str.replace(/-/g, '+').replace(/_/g, '/');
    // Add padding if necessary
    const pad = base64.length % 4;
    const padded = pad ? base64 + '='.repeat(4 - pad) : base64;
    // Decode Base64 to binary string
    let binary;
    try {
      binary = atob(padded);
    } catch (e) {
      throw new Error('Invalid base64 encoding in JWT.', { cause: e });
    }
    // Convert binary string to Uint8Array and decode as UTF-8
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
    return new TextDecoder().decode(bytes);
  }

  /**
   * Parses a newline-separated string into a list of non-empty trimmed values.
   * @param {string} text - The raw newline-separated input.
   * @returns {string[]} Array of trimmed, non-empty lines.
   */
  function parseTopicList(text) {
    return text
      .split('\n')
      .map((t) => t.trim())
      .filter((t) => t !== '');
  }

  /**
   * A cache of frequently accessed DOM elements for performance.
   */
  const DOM = {
    // Live Updates elements
    updatesEmpty: document.getElementById('updates-empty'),
    updatesList: document.getElementById('updates-list'),
    liveIndicator: document.getElementById('live-indicator'),
    eventCount: document.getElementById('event-count'),
    retryValue: document.getElementById('retry-value'),
    retryInput: document.getElementById('eventRetry'),
    retryDecrement: document.getElementById('retryDecrement'),
    retryIncrement: document.getElementById('retryIncrement'),
    // Subscriptions elements
    subscriptionsEmpty: document.getElementById('subscriptions-empty'),
    subscriptionsGrid: document.getElementById('subscriptions-grid'),
    subscriptionCountTotal: document.getElementById('subscription-count-total'),
    subscriptionCountClient: document.getElementById('subscription-count-client'),
    subscriptionCountMonitor: document.getElementById('subscription-count-monitor'),
    // Forms
    settingsForm: document.forms.settings,
    discoverForm: document.forms.discover,
    subscribeForm: document.forms.subscribe,
    publishForm: document.forms.publish,
    subscriptionsForm: document.forms.subscriptions,
    // Templates
    updateTemplate: document.getElementById('update'),
    subscriptionTemplate: document.getElementById('subscription'),
    // Other elements
    subscribeTopicsExamples: document.getElementById('subscribeTopicsExamples'),
    jwtHeader: document.getElementById('jwt-header'),
    jwtPayload: document.getElementById('jwt-payload'),
    copyPublicJwkButton: document.getElementById('copy-public-jwk'),
    copyPrivateJwkButton: document.getElementById('copy-private-jwk'),
    copyJwksButton: document.getElementById('copy-jwks'),
    copyPublicPemButton: document.getElementById('copy-public-pem'),
    copyJwtButton: document.getElementById('copy-jwt'),
    clearCookieButton: document.getElementById('clearCookie'),
    // Algorithm toggle
    algorithmWarning: document.getElementById('algorithm-warning'),
    // RS256 sections
    debugTokenRS256: document.getElementById('debug-token-rs256'),
    createTokenRS256: document.getElementById('create-token-rs256'),
    hubConfigRS256: document.getElementById('hub-config-rs256'),
    // HS256 sections
    debugTokenHS256: document.getElementById('debug-token-hs256'),
    createTokenHS256: document.getElementById('create-token-hs256'),
    hubConfigHS256: document.getElementById('hub-config-hs256'),
    // HS256 buttons and config preview
    copyHS256SecretButton: document.getElementById('copy-hs256-secret'),
    copyHS256SecretVerifyButton: document.getElementById('copy-hs256-secret-verify'),
    copyHS256ConfigButton: document.getElementById('copy-hs256-config'),
    hs256ConfigPreview: document.getElementById('hs256-config-preview'),
  };

  let updateCtrl;
  let subscriptionCtrl;
  let notificationTimeoutId = null;
  let hasCookie = false;
  let eventCounter = 0;

  /**
   * Updates all domain-dependent fields when the hub URL origin changes.
   * Replaces the origin in topics, data, discover URL, and syntax examples.
   * @param {string} newHubUrl - The new hub URL (e.g., "https://example.com/.well-known/mercure").
   * @param {object} [options] - Options.
   * @param {boolean} [options.skipDiscover=false] - Skip updating the Discover topic URL.
   */
  function propagateHubUrl(newHubUrl, { skipDiscover = false } = {}) {
    let newOrigin;
    try {
      newOrigin = new URL(newHubUrl).origin;
    } catch (e) {
      Logger.debug('[Settings] Could not propagate hub URL - invalid URL.', e);
      showNotification('Invalid hub URL.', 'warning', 2000);
      return;
    }

    const hubPath = '/.well-known/mercure';
    const placeholderTopic = `${newOrigin}${hubPath}/ui/demo/books/1.jsonld`;

    // Update Subscribe and Publish topic fields (newline-separated, apply per line)
    for (const field of [DOM.subscribeForm.topics, DOM.publishForm.topics]) {
      field.value = field.value
        .split('\n')
        .map((line) => replaceDomainInValue(line, newOrigin))
        .join('\n');
    }

    // Update Discover fields (skip when called from discovery success — those are user input)
    if (!skipDiscover) {
      DOM.discoverForm.topic.value = replaceDomainInValue(DOM.discoverForm.topic.value, newOrigin);
    }

    // Update @id in JSON body fields (discover body when not skipped, and publish data)
    const jsonFields = skipDiscover
      ? [DOM.publishForm.data]
      : [DOM.discoverForm.body, DOM.publishForm.data];
    for (const field of jsonFields) {
      try {
        const data = JSON.parse(field.value);
        if (data['@id']) {
          data['@id'] = replaceDomainInValue(data['@id'], newOrigin);
          field.value = JSON.stringify(data, null, 2);
        }
      } catch (e) {
        Logger.debug('[Settings] Field is not valid JSON, skipping @id update.', e);
      }
    }

    // Update syntax help examples
    DOM.subscribeTopicsExamples.textContent = `${newOrigin}${hubPath}/ui/demo/books/{id}.jsonld\n${placeholderTopic}`;

    Logger.debug(`[Settings] Propagated hub origin: ${newOrigin}`);
  }

  /**
   * Replaces the origin (scheme + host) in a URL string with a new origin.
   * @param {string} value - The original URL string.
   * @param {string} newOrigin - The new origin to use.
   * @returns {string} The URL with the replaced origin, or the original if not a valid URL.
   */
  function replaceDomainInValue(value, newOrigin) {
    try {
      new URL(value); // Validate it's a URL (throws for non-URLs like plain text)
      // Find where the path starts in the original string — first '/' after '://'.
      // This preserves the original path/query/hash verbatim, avoiding both
      // percent-encoding of URI template characters ({, }) and port normalization issues.
      const schemeEnd = value.indexOf('://');
      if (schemeEnd === -1) return value; // Non-hierarchical URIs (urn:, data:) — nothing to replace
      const pathStart = value.indexOf('/', schemeEnd + 3);
      const suffix = pathStart !== -1 ? value.slice(pathStart) : '/';
      return newOrigin.replace(/\/+$/, '') + suffix;
    } catch (e) {
      Logger.debug('[Settings] Value is not a URL, returning as-is.', { value, error: e.message });
      return value;
    }
  }

  /**
   * Enables or disables the Settings and Discover forms based on subscription state.
   * These forms should be disabled while subscribed to prevent confusing behavior
   * (changes won't take effect until resubscribe).
   */
  function updateConnectionDependentForms() {
    // Disable if either subscription is active (either unsubscribe button is enabled)
    const mainSubscribed = !DOM.subscribeForm.elements.unsubscribe.disabled;
    const presenceSubscribed = !DOM.subscriptionsForm.elements.unsubscribe.disabled;
    const disabled = mainSubscribed || presenceSubscribed;

    // Settings form elements
    DOM.settingsForm.hubUrl.disabled = disabled;
    DOM.settingsForm.jwt.disabled = disabled;

    for (const radio of DOM.settingsForm.querySelectorAll('input[name="authorization"]')) {
      radio.disabled = disabled;
    }
    DOM.clearCookieButton.disabled = disabled || !hasCookie;

    // Discover form elements
    DOM.discoverForm.topic.disabled = disabled;
    DOM.discoverForm.body.disabled = disabled;

    DOM.discoverForm.querySelector('button[name="discover"]').disabled = disabled;

    for (const radio of DOM.settingsForm.querySelectorAll('input[name="jwtAlgorithm"]')) {
      radio.disabled = disabled;
    }
  }

  /**
   * Updates UI sections based on selected JWT algorithm (RS256 or HS256).
   * Shows/hides appropriate content for Debug Token, Create New Token, and Hub Config.
   * Also shows a warning when HS256 is selected (since JWKS URL takes precedence).
   */
  function updateAlgorithmSections() {
    const isHS256 = DOM.settingsForm.jwtAlgorithm.value === 'hs256';

    // Show warning when HS256 selected (JWKS URL takes precedence, so hub config change is required)
    if (DOM.algorithmWarning) {
      DOM.algorithmWarning.style.display = isHS256 ? 'block' : 'none';
    }

    // Toggle RS256/HS256 sections: show one, hide the other
    const pairs = [
      [DOM.debugTokenRS256, DOM.debugTokenHS256],
      [DOM.createTokenRS256, DOM.createTokenHS256],
      [DOM.hubConfigRS256, DOM.hubConfigHS256],
    ];
    for (const [rs256El, hs256El] of pairs) {
      if (rs256El) rs256El.style.display = isHS256 ? 'none' : 'block';
      if (hs256El) hs256El.style.display = isHS256 ? 'block' : 'none';
    }

    // Display HS256 config preview if HS256 is selected
    if (isHS256 && DOM.hs256ConfigPreview) {
      DOM.hs256ConfigPreview.textContent = `subscriber_jwt ${CONFIG.UI_HS256_SECRET}\npublisher_jwt ${CONFIG.UI_HS256_SECRET}`;
    }
  }

  /**
   * Selects the demo token for the current algorithm and sets it in the JWT field.
   * @param {object} tokens - The token map (from cache or fetch).
   * @param {string} logPrefix - Log prefix for distinguishing callers (e.g., '[Init]', '[Auth]').
   */
  function applyTokenForCurrentAlgorithm(tokens, logPrefix) {
    const isHS256 = DOM.settingsForm.jwtAlgorithm?.value === 'hs256';
    const algLabel = isHS256 ? 'HS256' : 'RS256';
    const tokenKey = isHS256 ? CONFIG.UI_HS256_TOKEN_KEY : CONFIG.UI_DEMO_TOKEN_KEY;
    const token = tokens[tokenKey];
    if (!token) {
      Logger.warn(`${logPrefix} Token key "${tokenKey}" not found in cached tokens.`, tokens);
      showNotification(`Demo token for ${algLabel} not found in fixtures.`, 'warning');
    }
    DOM.settingsForm.jwt.value = token || '';
    Logger.debug(`${logPrefix} Applied ${algLabel} JWT token from cache.`);
  }

  /**
   * Loads the appropriate JWT token for the selected algorithm from cache.
   * Called when the algorithm toggle changes.
   */
  function loadTokenForAlgorithm() {
    if (!cachedDemoTokens) {
      Logger.warn('[Auth] No cached tokens available for algorithm switch.');
      showNotification('Tokens not loaded. Try refreshing the page.', 'warning');
      return;
    }
    applyTokenForCurrentAlgorithm(cachedDemoTokens, '[Auth]');
    updateJwtPayloadDisplay();
  }

  /**
   * Updates the Live Updates section UI state.
   * @param {'idle'|'connecting'|'active'|'waiting'} state - The current connection state.
   */
  function setLiveUpdatesState(state) {
    const indicator = DOM.liveIndicator;
    const empty = DOM.updatesEmpty;
    const list = DOM.updatesList;

    indicator.classList.remove('is-active');

    switch (state) {
      case 'idle':
        empty.style.display = 'flex';
        empty.querySelector('p').textContent = 'Not subscribed';
        empty.querySelector('span').textContent = 'Click Subscribe to start receiving events';
        list.innerHTML = '';
        eventCounter = 0;
        updateEventCount();
        DOM.retryValue.textContent = `${CONFIG.FES_RETRY_BASE_DELAY_MS}ms`;
        break;
      case 'connecting':
        empty.style.display = 'flex';
        empty.querySelector('p').textContent = 'Connecting...';
        empty.querySelector('span').textContent = 'Establishing SSE connection';
        break;
      case 'waiting':
        empty.style.display = 'flex';
        empty.querySelector('p').textContent = 'Waiting for events';
        empty.querySelector('span').textContent = 'Connection established, listening for updates';
        indicator.classList.add('is-active');
        break;
      case 'active':
        empty.style.display = 'none';
        indicator.classList.add('is-active');
        break;
    }
  }

  /**
   * Updates the event counter display.
   */
  function updateEventCount() {
    const text = eventCounter === 1 ? '1 event' : `${eventCounter} events`;
    DOM.eventCount.textContent = text;
  }

  /**
   * Updates the subscription counter display with total, client, and monitor counts.
   */
  function updateSubscriptionCount() {
    const cards = DOM.subscriptionsGrid.children;
    const total = cards.length;
    let clientCount = 0;
    let monitorCount = 0;

    for (const card of cards) {
      const badge = card.querySelector('.subscription-badge');
      if (badge?.classList.contains('is-monitor')) {
        monitorCount++;
      } else if (badge?.classList.contains('is-client')) {
        clientCount++;
      }
    }

    // Update total count
    DOM.subscriptionCountTotal.textContent = `${total} total`;

    // Update client count with label
    DOM.subscriptionCountClient.textContent = `${clientCount} client${clientCount !== 1 ? 's' : ''}`;
    DOM.subscriptionCountClient.dataset.count = clientCount.toString();

    // Update monitor count with label
    DOM.subscriptionCountMonitor.textContent = `${monitorCount} monitor${monitorCount !== 1 ? 's' : ''}`;
    DOM.subscriptionCountMonitor.dataset.count = monitorCount.toString();

    DOM.subscriptionsEmpty.style.display = total === 0 ? 'flex' : 'none';
  }

  /**
   * Updates the Clear Cookie button state based on whether a cookie is set.
   * @param {boolean} cookieSet - Whether the auth cookie is currently set.
   */
  function updateCookieButtonState(cookieSet) {
    hasCookie = cookieSet;
    DOM.clearCookieButton.disabled = !hasCookie;
  }

  /**
   * Displays a toast notification at the bottom of the page.
   * @param {string} message - The message to display.
   * @param {string} [type="error"] - The notification type ('error', 'success', 'info', 'warning').
   * @param {number} [duration=5000] - The duration in milliseconds to show the notification.
   */
  function showNotification(message, type = 'error', duration = 5000) {
    clearTimeout(notificationTimeoutId);
    const existingToast = document.querySelector('.app-notification');
    if (existingToast) existingToast.remove();

    // Map type to Bulma notification class (Bulma uses is-danger instead of is-error)
    const bulmaType = type === 'error' ? 'danger' : type;
    const notification = document.createElement('div');
    notification.className = `notification app-notification is-${bulmaType}`;
    notification.textContent = message;
    notification.setAttribute('role', type === 'error' ? 'alert' : 'status');
    notification.setAttribute('aria-live', type === 'error' ? 'assertive' : 'polite');
    document.body.appendChild(notification);

    setTimeout(() => notification.classList.add('is-visible'), 10);

    notificationTimeoutId = setTimeout(() => {
      notification.classList.remove('is-visible');
      notification.addEventListener('transitionend', () => notification.remove(), { once: true });
      setTimeout(() => {
        if (notification.parentNode) notification.remove();
      }, 500);
    }, duration);
  }

  /**
   * Clears validation error state from an input element.
   * @param {HTMLElement} input - The input element to clear.
   */
  function clearValidationError(input) {
    input.classList.remove('is-danger');
    input.removeAttribute('aria-invalid');
    input.removeAttribute('aria-describedby');
    if (input.hasAttribute('data-original-placeholder')) {
      input.placeholder = input.getAttribute('data-original-placeholder');
      input.removeAttribute('data-original-placeholder');
    }
    const errorHelp = input.closest('.field')?.querySelector('.help.is-danger.validation-error');
    if (errorHelp) errorHelp.remove();
  }

  /**
   * Shows a validation error on an input element.
   * @param {HTMLElement} input - The input element.
   * @param {string} message - The error message.
   * @param {boolean} [asPlaceholder=false] - Show message as placeholder instead of help text.
   */
  function showValidationError(input, message, asPlaceholder = false) {
    clearValidationError(input);
    input.classList.add('is-danger');
    input.setAttribute('aria-invalid', 'true');

    if (asPlaceholder) {
      input.setAttribute('data-original-placeholder', input.placeholder || '');
      input.placeholder = message;
    } else {
      const field = input.closest('.field');
      if (field) {
        const errorId = `${input.id || input.name}-error`;
        const helpText = document.createElement('p');
        helpText.id = errorId;
        helpText.className = 'help is-danger validation-error';
        helpText.textContent = message;
        field.appendChild(helpText);
        input.setAttribute('aria-describedby', errorId);
      }
    }
  }

  /**
   * Validates a single input element.
   * @param {HTMLElement} input - The input element to validate.
   * @returns {boolean} True if valid, false otherwise.
   */
  function validateInput(input) {
    clearValidationError(input);

    if (input.hasAttribute('required') && !input.value.trim()) {
      showValidationError(input, 'Required', true);
      return false;
    } else if (input.type === 'url' && input.value.trim()) {
      try {
        new URL(input.value.trim());
      } catch (err) {
        Logger.debug('[Validation] Invalid URL.', {
          value: input.value,
          error: err.message,
        });
        showValidationError(input, 'Please enter a valid URL (e.g., https://example.com)');
        return false;
      }
    }
    return true;
  }

  /**
   * Validates a form and returns whether it's valid.
   * Shows appropriate error messages for invalid fields.
   * @param {HTMLFormElement} form - The form to validate.
   * @param {Object} [options] - Validation options.
   * @param {boolean} [options.includeHubUrl=false] - Also validate Hub URL from settings.
   * @param {boolean} [options.includeJwt=false] - Also validate JWT from settings.
   * @returns {boolean} True if valid, false otherwise.
   */
  function validateForm(form, options = {}) {
    let isValid = true;

    // Validate each input (clearValidationError runs first inside validateInput)
    for (const input of form.querySelectorAll('input, textarea')) {
      if (!validateInput(input)) {
        isValid = false;
      }
    }

    // Also validate Hub URL from settings form if requested
    if (options.includeHubUrl) {
      if (!validateInput(DOM.settingsForm.hubUrl)) {
        isValid = false;
      }
    }

    // Also validate JWT from settings form if requested
    if (options.includeJwt) {
      if (!validateInput(DOM.settingsForm.jwt)) {
        isValid = false;
      }
    }

    return isValid;
  }

  /**
   * Extracts the Mercure Hub URL and self URL from the Link HTTP header.
   * Handles RFC 8288 compliant headers where parameters can appear in any order.
   * @param {Response} resp - The fetch response object.
   * @returns {{mercure: string, self?: string}} An object with the hub URL; self is optional.
   * @throws {Error} If the required Link headers are not found.
   */
  function parseLinkHeaders(resp) {
    const links = {};
    const linkHeader = resp.headers.get('Link');
    if (linkHeader) {
      // Split by comma, but only when followed by '<' to avoid splitting on commas in quoted values
      const linkValues = linkHeader.split(/,(?=\s*<)/);

      for (const linkValue of linkValues) {
        const urlMatch = linkValue.match(/<([^>]+)>/);
        // Match rel parameter anywhere in the link value (handles any parameter order)
        const relMatch = linkValue.match(/;\s*rel=["']?([^"';\s,]+)["']?/i);

        if (urlMatch && relMatch) {
          links[relMatch[1]] = urlMatch[1];
        }
      }
    }

    if (!links.mercure) {
      throw new Error('Invalid response from server: "mercure" Link header is required.');
    }

    return links;
  }

  /**
   * Creates a set of stateful callbacks for managing a resilient SSE connection.
   * This handles a two-stage retry strategy with stability reset: (1) uses the
   * server-sent delay for the first attempt, (2) falls back to exponential backoff
   * on subsequent attempts, and resets state after a stable connection (FES_RETRY_SUCCESS_RESET_MS).
   * @param {AbortSignal} signal - The AbortController signal to terminate the connection.
   * @param {object} [options={}] - Additional options.
   * @param {function} [options.onFatal=()=>{}] - Callback for fatal, non-retriable errors.
   * @param {string} [options.context='event stream'] - The context for logging and notifications.
   * @returns {object} An object containing onopen, onclose, onerror, and a captureRetry method.
   */
  function createStatefulSSECallbacks(
    signal,
    { onFatal = () => {}, context = 'event stream' } = {},
  ) {
    let retryCount = 0;
    let successfulConnectionTimeout;
    let lastServerRetry = null;

    const cleanup = () => clearTimeout(successfulConnectionTimeout);
    signal.addEventListener('abort', () => {
      Logger.debug(`[SSE] Abort signal received for ${context}.`);
      cleanup();
    });

    return {
      /**
       * Captures the server-sent retry value from an SSE message, if present.
       * Returns the most recent server-sent value, or falls back to the default base delay.
       * @param {object} event - The message event from the SSE stream.
       * @returns {number} The effective retry value (server-sent or default).
       */
      captureRetry(event) {
        if (typeof event.retry === 'number') {
          lastServerRetry = event.retry;
          Logger.debug(`[SSE] Server-sent retry: ${lastServerRetry}ms.`);
          return lastServerRetry;
        }
        // No retry field in this event — preserve any previously received server value.
        return lastServerRetry ?? CONFIG.FES_RETRY_BASE_DELAY_MS;
      },

      async onopen(response) {
        if (response.ok && response.headers.get('content-type')?.includes('text/event-stream')) {
          Logger.info(`[SSE] Connected to ${context}.`);
          showNotification(`Connected to ${context}.`, 'success', 3000);

          cleanup();
          successfulConnectionTimeout = setTimeout(() => {
            // After a stable connection, reset the state for the next failure cycle.
            retryCount = 0;
            lastServerRetry = null;
            Logger.debug(`[SSE] Connection to ${context} stable, retry state reset.`);
          }, CONFIG.FES_RETRY_SUCCESS_RESET_MS);
          return;
        }
        if (response.status >= 400 && response.status < 500 && response.status !== 429) {
          const errorBody = await response.text();
          throw new FatalError(
            `Client-side error: ${response.status} ${response.statusText} - ${errorBody}`,
          );
        }
        throw new RetriableError();
      },

      onclose() {
        cleanup();
        Logger.warn(`[SSE] Connection to ${context} closed by server, will reconnect.`);
        throw new RetriableError();
      },

      onerror(err) {
        cleanup();
        if (err instanceof FatalError) {
          Logger.error(`[SSE] Fatal error on ${context}, halting retries.`, err);
          showNotification(
            `Failed to subscribe to ${context}: ${formatErrorMessage(err)}.`,
            'error',
          );
          onFatal();
          throw err;
        }

        let retryInterval;
        const jitter = (Math.random() - 0.5) * CONFIG.FES_RETRY_BASE_DELAY_MS;

        // Stage 1: Use server-sent value for the first attempt only.
        if (retryCount === 0 && lastServerRetry !== null) {
          Logger.debug(`[SSE] Using server-sent delay: ${lastServerRetry}ms.`);
          // The server's value is the effective max; we just ensure it's not below our absolute minimum.
          retryInterval = Math.max(CONFIG.FES_RETRY_BASE_DELAY_MS, lastServerRetry + jitter);
        } else {
          // Stage 2: Use client-side exponential backoff, capped at 30 seconds.
          const baseDelay = CONFIG.FES_RETRY_BASE_DELAY_MS * 2 ** retryCount;
          retryInterval = Math.min(
            CONFIG.FES_RETRY_MAX_DELAY_MS,
            Math.max(CONFIG.FES_RETRY_BASE_DELAY_MS, baseDelay + jitter),
          );
        }

        retryCount++;
        Logger.warn(
          `[SSE] Connection to ${context} lost, retry #${retryCount} in ${Math.round(retryInterval)}ms.`,
          err,
        );
        showNotification(
          `Connection to ${context} lost. Retrying (attempt ${retryCount})...`,
          'info',
          2000,
        );
        return retryInterval;
      },
    };
  }

  /**
   * Gets the appropriate fetch options based on the selected authorization type (Header or Cookie).
   * @param {object} [overrides={}] - Optional overrides for auth behavior.
   * @param {boolean} [overrides.anonymous=false] - If true, returns empty options (no auth).
   * @returns {object} Fetch options containing headers or credentials.
   */
  function getAuthOptions({ anonymous = false } = {}) {
    if (anonymous) {
      return { credentials: 'omit' };
    }
    const options = {};
    const authType = DOM.settingsForm.authorization.value;
    if (authType === 'header') {
      options.headers = {
        Authorization: `Bearer ${DOM.settingsForm.jwt.value}`,
      };
    } else if (authType === 'cookie') {
      options.credentials = 'include';
    }
    return options;
  }

  /**
   * Formats an error message for user display.
   * Replaces technical "Failed to fetch" with a clearer network/CORS message.
   * @param {Error} err - The error object.
   * @returns {string} User-friendly error message.
   */
  function formatErrorMessage(err) {
    const msg = err.message?.trim() || 'Unknown error';
    return msg.toLowerCase().includes('failed to fetch')
      ? 'Could not reach hub (network error or CORS)'
      : msg;
  }

  /**
   * Opens jwt.io in a new tab with the current JWT pre-filled.
   * Must be called synchronously in the user gesture context (before any await)
   * to avoid popup blockers in Safari and Firefox.
   */
  function openJwtIo() {
    const token = DOM.settingsForm.jwt.value;
    const jwtUrl = token ? `https://jwt.io/#token=${encodeURIComponent(token)}` : 'https://jwt.io/';
    window.open(jwtUrl, '_blank', 'noopener');
  }

  /**
   * Optionally opens jwt.io, then fetches text from a URL, copies it to
   * clipboard, and shows a notification.
   * @param {string} url - The fixture URL to fetch.
   * @param {string} successMessage - Notification text on success.
   * @param {object} [options]
   * @param {boolean} [options.withJwtIo=false] - Also open jwt.io (before async work, to avoid popup blockers).
   */
  async function fetchAndCopy(url, successMessage, { withJwtIo = false } = {}) {
    if (withJwtIo) openJwtIo();
    let text;
    try {
      const resp = await fetch(url);
      if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
      text = await resp.text();
    } catch (err) {
      showNotification(`Failed to load file: ${formatErrorMessage(err)}.`, 'error');
      Logger.error('[Fetch] Failed to load file.', { url, error: err });
      return;
    }
    await copyAndNotify(text, successMessage);
  }

  /**
   * Optionally opens jwt.io, then copies a string to clipboard and shows a notification.
   * @param {string} text - The text to copy.
   * @param {string} successMessage - Notification text on success.
   * @param {object} [options]
   * @param {boolean} [options.withJwtIo=false] - Also open jwt.io (before async work, to avoid popup blockers).
   */
  async function copyAndNotify(text, successMessage, { withJwtIo = false } = {}) {
    if (withJwtIo) openJwtIo();
    try {
      await navigator.clipboard.writeText(text);
      showNotification(successMessage, 'success', 4000);
    } catch (err) {
      showNotification('Failed to copy to clipboard.', 'error');
      Logger.error('[Clipboard] Failed to write to clipboard.', err);
    }
  }

  /**
   * Makes an element keyboard-accessible as a button (tabindex, role, Enter/Space).
   * @param {HTMLElement} el - The element to make accessible.
   */
  function makeClickable(el) {
    el.setAttribute('tabindex', '0');
    el.setAttribute('role', 'button');
    el.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        el.click();
      }
    });
  }

  /**
   * Copies text to clipboard with a brief success/failure notification.
   * @param {string} text - The text to copy.
   * @param {string} label - Human-readable label (e.g., "Topic", "Event ID").
   */
  async function copyToClipboard(text, label) {
    try {
      await navigator.clipboard.writeText(text);
      showNotification(`${label} copied.`, 'success', 1500);
    } catch (err) {
      showNotification(`Failed to copy ${label.toLowerCase()}.`, 'warning', 1500);
      Logger.warn(`[Clipboard] Failed to copy ${label.toLowerCase()}.`, err);
    }
  }

  /**
   * Handles the submission of the "Discover" form. Fetches a topic URL, parses
   * the Link headers, and auto-populates the Hub URL and topic fields.
   * @param {Event} e - The form submission event.
   */
  async function handleDiscoverSubmit(e) {
    e.preventDefault();
    if (!validateForm(e.target, { includeHubUrl: false })) return;
    const {
      elements: { topic, body },
    } = e.target;
    const jwt = DOM.settingsForm.jwt.value;
    let url;
    try {
      url = new URL(topic.value);
    } catch (err) {
      showNotification('Invalid topic URL.', 'error');
      Logger.debug('[Discovery] Invalid topic URL.', { value: topic.value, error: err });
      return;
    }
    if (body.value) url.searchParams.append('body', body.value);
    // Demo handler reads ?jwt= to set the auth cookie (demo.go). This is distinct from
    // the Mercure spec's ?authorization= param and RFC 6750's ?access_token= param.
    if (jwt) {
      url.searchParams.append('jwt', jwt);
    } else {
      Logger.warn('[Discovery] No JWT provided - auth cookie will be cleared.');
      showNotification('No JWT provided - auth cookie will be cleared.', 'warning', 3000);
    }

    Logger.info(`[Discovery] Fetching topic: ${url.href}`);
    try {
      const resp = await fetch(url, { credentials: 'include' });
      if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);

      const links = parseLinkHeaders(resp);
      // Fall back to topic URL per spec: "If the Link with rel=self is omitted,
      // the current URL of the resource MUST be used as a fallback."
      const selfUrl = new URL(links.self || topic.value, topic.value);
      const cleanSelfUrl = selfUrl.origin + selfUrl.pathname;

      // Auto-populate hub URL and propagate the new origin to all fields
      // Skip discover fields — the user's topic URL triggered this discovery
      DOM.settingsForm.hubUrl.value = new URL(links.mercure, topic.value);
      propagateHubUrl(DOM.settingsForm.hubUrl.value, { skipDiscover: true });

      body.value = await resp.text();
      showNotification('Discovery successful.', 'success');
      Logger.info(
        `[Discovery] Success. Hub: ${DOM.settingsForm.hubUrl.value}, Topic: ${cleanSelfUrl}`,
      );
      DOM.settingsForm.authorization.value = 'cookie';

      // Track cookie state: cookie is set if JWT was provided, deleted if not
      updateCookieButtonState(!!jwt);
    } catch (err) {
      showNotification(`Discovery failed: ${formatErrorMessage(err)}.`, 'error');
      Logger.error('[Discovery] Failed.', err);
    } finally {
      // Re-enable cookie radio (disabled during cookie-radio-triggered discovery)
      const cookieRadio = DOM.settingsForm.querySelector(
        'input[name="authorization"][value="cookie"]',
      );
      if (cookieRadio) cookieRadio.disabled = false;
    }
  }

  /**
   * Handles the submission of the main "Subscribe" form.
   * Establishes a persistent SSE connection to the Mercure hub for general updates.
   * @param {Event} e - The form submission event.
   */
  function handleSubscribeSubmit(e) {
    e.preventDefault();
    const isAnonymous = e.target.elements.anonymous.checked;
    const needsJwt = !isAnonymous && DOM.settingsForm.authorization.value === 'header';
    if (!validateForm(e.target, { includeHubUrl: true, includeJwt: needsJwt })) return;
    if (updateCtrl) updateCtrl.abort();
    updateCtrl = new AbortController();

    const {
      elements: { topics, lastEventId, subscribe, unsubscribe },
    } = e.target;
    let u;
    try {
      u = new URL(DOM.settingsForm.hubUrl.value);
    } catch (err) {
      showNotification('Invalid hub URL.', 'error');
      Logger.debug('[Subscribe] Invalid hub URL.', {
        value: DOM.settingsForm.hubUrl.value,
        error: err,
      });
      return;
    }
    const topicList = parseTopicList(topics.value);
    for (const topic of topicList) {
      u.searchParams.append('topic', topic);
    }
    if (lastEventId.value) {
      u.searchParams.append('lastEventID', lastEventId.value);
    }

    Logger.info('[SSE] Subscribing to main event stream.', {
      topics: topicList,
      lastEventId: lastEventId.value || '(none)',
      anonymous: isAnonymous,
    });
    showNotification(
      isAnonymous ? 'Connecting anonymously...' : 'Connecting to main event stream...',
      'info',
    );
    setLiveUpdatesState('connecting');

    const { onopen, onclose, onerror, captureRetry } = createStatefulSSECallbacks(
      updateCtrl.signal,
      {
        context: 'main event stream',
        onFatal: () => {
          subscribe.disabled = false;
          unsubscribe.disabled = true;
          e.target.elements.anonymous.disabled = false;
          e.target.elements.lastEventId.disabled = false;
          e.target.elements.openWhenHidden.disabled = false;
          setLiveUpdatesState('idle');
          updateConnectionDependentForms();
        },
      },
    );

    const options = {
      ...getAuthOptions({ anonymous: isAnonymous }),
      signal: updateCtrl.signal,
      onopen: async (response) => {
        await onopen(response);
        if (response.ok && DOM.updatesList.children.length === 0) {
          setLiveUpdatesState('waiting');
        }
      },
      onmessage(event) {
        // Capture retry and update UI indicator
        const effectiveRetry = captureRetry(event);
        DOM.retryValue.textContent = `${effectiveRetry}ms`;

        Logger.debug('[SSE] Received event.', {
          id: event.id,
          type: event.event || '(default)',
          dataLength: event.data?.length ?? 0,
        });
        if (event.event === 'FatalError') throw new FatalError(event.data);

        // Show all events including those with empty data (falsy "" would be ignored otherwise)
        if (event.data !== undefined) {
          setLiveUpdatesState('active');

          const li = document.importNode(DOM.updateTemplate.content, true);

          // Event ID with click-to-copy
          const idEl = li.querySelector('.event-id');
          idEl.textContent = event.id;
          idEl.title = 'Click to copy';
          makeClickable(idEl);
          idEl.addEventListener('click', (e) => {
            e.stopPropagation();
            copyToClipboard(event.id, 'Event ID');
          });

          // Type badge
          const typeTag = li.querySelector('.event-type');
          if (event.event) {
            typeTag.textContent = event.event;
          } else {
            typeTag.style.display = 'none';
          }

          // Data display with JSON formatting
          const codeBlock = li.querySelector('.event-data code');
          if (event.data === '') {
            // Empty data is valid (signal-only event) - show indicator
            codeBlock.textContent = '(no data)';
            codeBlock.classList.add('is-empty-data');
          } else {
            let displayData = event.data;
            let isJson = false;
            try {
              const parsed = JSON.parse(event.data);
              displayData = JSON.stringify(parsed, null, 2);
              isJson = true;
            } catch (err) {
              Logger.debug('[SSE] Event data is not JSON, displaying as plain text.', err);
            }
            codeBlock.textContent = displayData;
            if (isJson && window.hljs) {
              codeBlock.classList.add('language-json');
              hljs.highlightElement(codeBlock);
            }
          }

          // Add flash animation for new events
          const eventItem = li.querySelector('.event-item');
          eventItem.classList.add('is-new');

          const list = DOM.updatesList;
          list.firstChild ? list.insertBefore(li, list.firstChild) : list.appendChild(li);

          eventCounter++;
          updateEventCount();

          // Cap DOM size to prevent memory exhaustion during long sessions
          while (list.children.length > 100) {
            list.removeChild(list.lastChild);
          }
        }
      },
      onclose,
      onerror,
      openWhenHidden: e.target.elements.openWhenHidden.checked,
    };

    fetchEventSource(u, options).catch((err) => {
      if (updateCtrl.signal.aborted) {
        Logger.debug('[SSE] fetchEventSource aborted.', err);
        return;
      }
      if (err instanceof FatalError) return; // Already handled in onerror
      Logger.error('[SSE] Unexpected fetchEventSource rejection.', err);
      showNotification('Unexpected error in event stream. Check console.', 'error');
    });

    subscribe.disabled = true;
    unsubscribe.disabled = false;
    e.target.elements.anonymous.disabled = true;
    e.target.elements.lastEventId.disabled = true;
    e.target.elements.openWhenHidden.disabled = true;
    updateConnectionDependentForms();
  }

  /**
   * Handles the submission of the "Publish" form.
   * Sends a POST request to the Mercure hub to dispatch an update.
   * @param {Event} e - The form submission event.
   */
  async function handlePublishSubmit(e) {
    e.preventDefault();
    const needsJwt = DOM.settingsForm.authorization.value === 'header';
    if (!validateForm(e.target, { includeHubUrl: true, includeJwt: needsJwt })) return;
    const {
      elements: { topics, data, priv, id, type, retry },
    } = e.target;
    const body = new URLSearchParams({ data: data.value });
    if (id.value) body.append('id', id.value);
    if (type.value) body.append('type', type.value);
    if (retry.value) body.append('retry', retry.value);
    const topicList = parseTopicList(topics.value);
    for (const topic of topicList) {
      body.append('topic', topic);
    }
    if (priv.checked) body.append('private', 'on');

    const options = { ...getAuthOptions(), method: 'POST', body };

    Logger.info('[Publish] Sending update.', {
      topics: topicList,
      private: priv.checked,
    });
    try {
      const resp = await fetch(DOM.settingsForm.hubUrl.value, options);
      if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
      const eventId = await resp.text();
      showNotification('Update published.', 'success');
      Logger.info(`[Publish] Success. Event ID: ${eventId.trim() || '(not returned)'}`);
    } catch (err) {
      showNotification(`Publish failed: ${formatErrorMessage(err)}.`, 'error');
      Logger.error('[Publish] Failed.', err);
    }
  }

  /**
   * Renders a subscription card using the template and appends it to the container.
   * @param {object} s - The subscription data object.
   */
  function renderSubscriptionCard(s) {
    const card = document.importNode(DOM.subscriptionTemplate.content, true);
    const article = card.querySelector('article');
    article.id = s.id;
    const isMonitor = s.topic.includes('/.well-known/mercure/subscriptions');

    // Topic in header (primary identifier) - click to copy
    const topicWrapper = article.querySelector('.subscription-topic-wrapper');
    const topicEl = article.querySelector('.subscription-topic');
    topicEl.textContent = s.topic;
    const truncatedTopic = s.topic.length > 60 ? `${s.topic.slice(0, 60)}…` : s.topic;
    topicWrapper.dataset.tooltip = `${truncatedTopic}\n\nClick to copy`;
    makeClickable(topicEl);
    topicEl.addEventListener('click', () => copyToClipboard(s.topic, 'Topic'));

    // Badge to distinguish subscription types
    const badge = article.querySelector('.subscription-badge');
    if (isMonitor) {
      badge.textContent = 'Monitor';
      badge.classList.add('is-monitor');
    } else {
      badge.textContent = 'Client';
      badge.classList.add('is-client');
    }

    // Subscriber in body (secondary) - click to copy
    const subEl = article.querySelector('.subscription-subscriber');
    subEl.textContent = s.subscriber;
    const truncatedSub = s.subscriber.length > 60 ? `${s.subscriber.slice(0, 60)}…` : s.subscriber;
    subEl.dataset.tooltip = `${truncatedSub}\n\nClick to copy`;
    makeClickable(subEl);
    subEl.addEventListener('click', () => copyToClipboard(s.subscriber, 'Subscriber'));

    // Payload is optional (from JWT mercure.payload claim)
    const payloadDetails = article.querySelector('.subscription-payload-details');
    if (s.payload && Object.keys(s.payload).length > 0) {
      const payloadCode = article.querySelector('.subscription-payload code');
      payloadCode.textContent = JSON.stringify(s.payload, null, 2);
      if (window.hljs) hljs.highlightElement(payloadCode);
    } else {
      payloadDetails.remove();
    }

    // Insert sorted: monitors first, then clients
    if (isMonitor) {
      // Insert before the first client card (or append if none)
      const firstClient = DOM.subscriptionsGrid
        .querySelector('.is-client')
        ?.closest('.subscription-card');
      DOM.subscriptionsGrid.insertBefore(card, firstClient || null);
    } else {
      DOM.subscriptionsGrid.appendChild(card);
    }
    updateSubscriptionCount();
  }

  /**
   * Handles the submission of the "Active Subscriptions" form to listen for presence events.
   * @param {Event} e - The form submission event.
   */
  async function handleActiveSubscriptionsSubmit(e) {
    e.preventDefault();
    // Active Subscriptions needs Hub URL and JWT (no form fields of its own)
    const needsJwt = DOM.settingsForm.authorization.value === 'header';
    if (
      !validateInput(DOM.settingsForm.hubUrl) ||
      (needsJwt && !validateInput(DOM.settingsForm.jwt))
    )
      return;
    if (subscriptionCtrl) subscriptionCtrl.abort();
    subscriptionCtrl = new AbortController();
    const {
      elements: { subscribe, unsubscribe },
    } = e.target;

    DOM.subscriptionsGrid.innerHTML = '';
    updateSubscriptionCount();
    showNotification('Connecting to active subscriptions stream...', 'info');
    Logger.info('[Presence] Fetching initial subscriptions snapshot.');

    try {
      const fetchOptions = {
        ...getAuthOptions(),
        signal: subscriptionCtrl.signal,
      };
      const resp = await fetch(`${DOM.settingsForm.hubUrl.value}/subscriptions`, fetchOptions);
      if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
      const json = await resp.json();
      for (const subscription of json.subscriptions) {
        renderSubscriptionCard(subscription);
      }
      Logger.info(`[Presence] Loaded ${json.subscriptions.length} active subscriptions.`);

      const u = new URL(DOM.settingsForm.hubUrl.value);
      u.searchParams.append('topic', '/.well-known/mercure/subscriptions{/topic}{/subscriber}');
      if (json.lastEventID) {
        u.searchParams.append('lastEventID', json.lastEventID);
      }

      Logger.info('[Presence] Subscribing to live updates.', {
        lastEventId: json.lastEventID || '(none)',
      });

      const { onopen, onclose, onerror, captureRetry } = createStatefulSSECallbacks(
        subscriptionCtrl.signal,
        {
          context: 'active subscriptions stream',
          onFatal: () => {
            subscribe.disabled = false;
            unsubscribe.disabled = true;
            updateConnectionDependentForms();
          },
        },
      );

      const sseOptions = {
        ...fetchOptions,
        onopen,
        onmessage(event) {
          captureRetry(event);
          Logger.debug('[Presence] Received event.', {
            id: event.id,
            dataLength: event.data?.length ?? 0,
          });
          if (event.event === 'FatalError') throw new FatalError(event.data);

          // Subscription events require valid JSON data; empty strings are not valid JSON
          if (event.data) {
            let s;
            try {
              s = JSON.parse(event.data);
            } catch (e) {
              Logger.error('[Presence] Failed to parse event data as JSON.', e, {
                eventId: event.id,
                raw: event.data,
              });
              showNotification(
                `Received malformed subscription data (event ${event.id || 'unknown'}).`,
                'warning',
              );
              return;
            }
            const existingSub = document.getElementById(s.id);
            if (s.active) {
              // Refresh card if metadata changed, or create new if doesn't exist
              if (existingSub) {
                const existingPayload =
                  existingSub.querySelector('.subscription-payload code')?.textContent ?? null;
                const hasPayload = s.payload && Object.keys(s.payload).length > 0;
                const newPayload = hasPayload ? JSON.stringify(s.payload, null, 2) : null;
                if (existingPayload !== newPayload) {
                  existingSub.remove();
                  renderSubscriptionCard(s);
                }
              } else {
                renderSubscriptionCard(s);
              }
            } else {
              if (existingSub) {
                existingSub.remove();
                updateSubscriptionCount();
              }
            }
          }
        },
        onclose,
        onerror,
        openWhenHidden: CONFIG.FES_OPEN_WHEN_HIDDEN_PRESENCE,
      };

      fetchEventSource(u, sseOptions).catch((err) => {
        if (subscriptionCtrl.signal.aborted) {
          Logger.debug('[Presence] fetchEventSource aborted.', err);
          return;
        }
        if (err instanceof FatalError) return; // Already handled in onerror
        Logger.error('[Presence] Unexpected fetchEventSource rejection.', err);
        showNotification('Unexpected error in subscription stream. Check console.', 'error');
      });

      subscribe.disabled = true;
      unsubscribe.disabled = false;
      updateConnectionDependentForms();
    } catch (err) {
      if (err.name === 'AbortError') {
        Logger.debug('[Presence] Initial subscription fetch aborted.', err);
        return;
      }
      showNotification(`Could not retrieve subscriptions: ${formatErrorMessage(err)}.`, 'error');
      Logger.error('[Presence] Failed to retrieve or subscribe.', err);
      subscribe.disabled = false;
      unsubscribe.disabled = true;
      updateConnectionDependentForms();
    }
  }

  /**
   * Decodes a JWT and displays its header and payload in the UI.
   * Handles empty, valid, and malformed tokens gracefully.
   */
  function updateJwtPayloadDisplay() {
    const token = DOM.settingsForm.jwt.value;
    const headerContainer = DOM.jwtHeader;
    const payloadContainer = DOM.jwtPayload;

    // Reset UI state (remove hljs class to allow re-highlighting)
    headerContainer.textContent = '';
    payloadContainer.textContent = '';
    headerContainer.classList.remove('has-text-danger', 'hljs', 'language-json');
    payloadContainer.classList.remove('has-text-danger', 'hljs', 'language-json');
    delete headerContainer.dataset.highlighted;
    delete payloadContainer.dataset.highlighted;

    if (!token) {
      headerContainer.textContent = 'Enter a JWT to see its header.';
      payloadContainer.textContent = 'Enter a JWT to see its payload.';
      headerContainer.classList.add('has-text-danger');
      payloadContainer.classList.add('has-text-danger');
      return;
    }

    const parts = token.split('.');
    if (parts.length !== 3) {
      const errorMsg = 'Malformed JWT: Must contain 3 parts separated by dots.';
      headerContainer.textContent = errorMsg;
      payloadContainer.textContent = errorMsg;
      headerContainer.classList.add('has-text-danger');
      payloadContainer.classList.add('has-text-danger');
      Logger.warn(`[Auth] Invalid JWT structure: expected 3 parts, got ${parts.length}.`);
      return;
    }

    const [headerB64, payloadB64] = parts;
    decodeAndDisplayJwtPart(headerB64, headerContainer, 'Header');
    decodeAndDisplayJwtPart(payloadB64, payloadContainer, 'Payload');
  }

  /**
   * Decodes a Base64URL-encoded JWT part and renders it as highlighted JSON.
   * @param {string} base64Str - The Base64URL-encoded string to decode.
   * @param {HTMLElement} container - The element to render into.
   * @param {string} label - Human-readable label (e.g., "Header", "Payload").
   */
  function decodeAndDisplayJwtPart(base64Str, container, label) {
    try {
      const decoded = decodeBase64Url(base64Str);
      const parsed = JSON.parse(decoded);
      container.textContent = JSON.stringify(parsed, null, 2);
      if (window.hljs) {
        container.classList.add('language-json');
        hljs.highlightElement(container);
      }
    } catch (e) {
      container.textContent = `Malformed ${label}: Could not decode.`;
      container.classList.add('has-text-danger');
      Logger.warn(`[Auth] Failed to decode JWT ${label.toLowerCase()}.`, e);
    }
  }

  /**
   * Populates forms with default values on application startup using CONFIG constants.
   * @returns {Promise<void>}
   */
  async function setDefaultValues() {
    DOM.settingsForm.hubUrl.value = CONFIG.UI_DEFAULT_HUB_URL;

    // Fetch demo tokens (cached after first call)
    const tokens = await fetchDemoTokens();
    if (tokens) {
      applyTokenForCurrentAlgorithm(tokens, '[Init]');
    } else {
      DOM.settingsForm.jwt.value = '';
    }

    // Set default topic for Subscribe and Publish fields
    DOM.subscribeForm.topics.value = CONFIG.UI_PLACEHOLDER_TOPIC;
    DOM.publishForm.topics.value = CONFIG.UI_PLACEHOLDER_TOPIC;

    // Set default topic and body for the Discover feature
    DOM.discoverForm.topic.value = CONFIG.UI_PLACEHOLDER_TOPIC;
    DOM.discoverForm.body.value = JSON.stringify(
      {
        '@id': CONFIG.UI_PLACEHOLDER_TOPIC,
        availability: 'https://schema.org/InStock',
      },
      null,
      2,
    );

    // Set default data for the Publish feature
    DOM.publishForm.data.value = JSON.stringify(
      {
        '@id': CONFIG.UI_PLACEHOLDER_TOPIC,
        availability: 'https://schema.org/OutOfStock',
      },
      null,
      2,
    );

    // Set example topics in the help text
    DOM.subscribeTopicsExamples.textContent = `${CONFIG.UI_DEFAULT_HUB_URL}/ui/demo/books/{id}.jsonld\n${CONFIG.UI_PLACEHOLDER_TOPIC}`;
  }

  /**
   * Attaches all primary event listeners to the DOM elements.
   */
  function initializeEventListeners() {
    Logger.debug('[Init] Attaching event listeners.');
    DOM.discoverForm.addEventListener('submit', handleDiscoverSubmit);
    DOM.subscribeForm.addEventListener('submit', handleSubscribeSubmit);
    DOM.publishForm.addEventListener('submit', handlePublishSubmit);
    DOM.subscriptionsForm.addEventListener('submit', handleActiveSubscriptionsSubmit);
    DOM.settingsForm.jwt.addEventListener('input', updateJwtPayloadDisplay);

    // Initialize retry indicator with default value
    DOM.retryValue.textContent = `${CONFIG.FES_RETRY_BASE_DELAY_MS}ms`;

    // Clear validation errors when user starts typing
    for (const input of document.querySelectorAll('input, textarea')) {
      input.addEventListener('input', () => clearValidationError(input));
    }

    const cookieAuthRadio = DOM.settingsForm.querySelector(
      'input[name="authorization"][value="cookie"]',
    );
    // Propagate hub URL origin to all fields when manually changed
    DOM.settingsForm.hubUrl.addEventListener('change', () => {
      propagateHubUrl(DOM.settingsForm.hubUrl.value);
    });

    cookieAuthRadio.addEventListener('click', (e) => {
      // Prevent the radio from selecting until discovery succeeds.
      // handleDiscoverSubmit sets authorization.value = 'cookie' on success.
      e.preventDefault();
      if (!validateInput(DOM.settingsForm.jwt)) return;
      cookieAuthRadio.disabled = true;
      DOM.discoverForm.requestSubmit();
    });

    const headerAuthRadio = DOM.settingsForm.querySelector(
      'input[name="authorization"][value="header"]',
    );
    headerAuthRadio.addEventListener('click', () => {
      if (!hasCookie) return;
      // Clear stale cookie when switching to Header mode to prevent it from interfering
      DOM.clearCookieButton.click();
    });

    // Clear Cookie button - triggers Discover without JWT to delete the auth cookie
    DOM.clearCookieButton.addEventListener('click', async () => {
      const topicUrl = DOM.discoverForm.topic.value;
      if (!topicUrl) {
        showNotification('Enter a topic URL first to clear the cookie.', 'warning');
        return;
      }

      Logger.info('[Auth] Clearing cookie via discovery without JWT.');
      try {
        const resp = await fetch(topicUrl, { credentials: 'include' });
        if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
        updateCookieButtonState(false);
        showNotification('Auth cookie cleared.', 'success');
        Logger.info('[Auth] Cookie cleared successfully.');
      } catch (err) {
        showNotification(`Failed to clear cookie: ${formatErrorMessage(err)}.`, 'error');
        Logger.error('[Auth] Failed to clear cookie.', err);
      }
    });

    // RS256 copy buttons (fetch fixture file → clipboard)
    DOM.copyPublicJwkButton.addEventListener('click', () =>
      fetchAndCopy(
        CONFIG.UI_PUBLIC_JWK_PATH,
        "Public JWK copied. Paste in jwt.io's signature section.",
        {
          withJwtIo: true,
        },
      ),
    );
    DOM.copyPrivateJwkButton.addEventListener('click', () =>
      fetchAndCopy(
        CONFIG.UI_PRIVATE_JWK_PATH,
        "Private JWK copied. Paste in jwt.io's 'Sign JWT' field.",
        {
          withJwtIo: true,
        },
      ),
    );
    DOM.copyJwksButton.addEventListener('click', () =>
      fetchAndCopy(CONFIG.UI_JWKS_PATH, 'JWKS copied. Host it and point publisher_jwks_url at it.'),
    );
    DOM.copyPublicPemButton.addEventListener('click', () =>
      fetchAndCopy(
        CONFIG.UI_PUBLIC_PEM_PATH,
        'PEM public key copied. Use with publisher_jwt + RS256.',
      ),
    );

    // Copy current JWT token
    DOM.copyJwtButton.addEventListener('click', async () => {
      const token = DOM.settingsForm.jwt.value;
      if (!token) {
        showNotification('No token to copy.', 'warning');
        return;
      }
      await copyAndNotify(token, 'Token copied.');
    });

    // Algorithm toggle - load appropriate token from cache and update UI sections

    for (const radio of DOM.settingsForm.querySelectorAll('input[name="jwtAlgorithm"]')) {
      radio.addEventListener('change', () => {
        updateAlgorithmSections();
        loadTokenForAlgorithm();
      });
    }

    // HS256 copy buttons (copy constant → clipboard)
    DOM.copyHS256SecretButton.addEventListener('click', () =>
      copyAndNotify(
        CONFIG.UI_HS256_SECRET,
        "Secret copied. Paste in jwt.io's SIGN JWT: SECRET field.",
        {
          withJwtIo: true,
        },
      ),
    );
    DOM.copyHS256SecretVerifyButton.addEventListener('click', () =>
      copyAndNotify(CONFIG.UI_HS256_SECRET, "Secret copied. Paste in jwt.io's SECRET field.", {
        withJwtIo: true,
      }),
    );
    DOM.copyHS256ConfigButton.addEventListener('click', () =>
      copyAndNotify(
        CONFIG.UI_HS256_SECRET,
        'Secret copied. Add to publisher_jwt and subscriber_jwt config.',
      ),
    );

    DOM.subscribeForm.elements.unsubscribe.addEventListener('click', (e) => {
      e.preventDefault();
      if (updateCtrl) updateCtrl.abort();
      e.currentTarget.disabled = true;
      DOM.subscribeForm.elements.subscribe.disabled = false;
      DOM.subscribeForm.elements.anonymous.disabled = false;
      DOM.subscribeForm.elements.lastEventId.disabled = false;
      DOM.subscribeForm.elements.openWhenHidden.disabled = false;
      setLiveUpdatesState('idle');
      updateConnectionDependentForms();
      showNotification('Unsubscribed from main event stream.', 'info');
      Logger.info('[SSE] Unsubscribed from main event stream.');
    });

    DOM.subscriptionsForm.elements.unsubscribe.addEventListener('click', (e) => {
      e.preventDefault();
      if (subscriptionCtrl) subscriptionCtrl.abort();
      e.currentTarget.disabled = true;
      DOM.subscriptionsForm.elements.subscribe.disabled = false;
      DOM.subscriptionsGrid.innerHTML = '';
      updateSubscriptionCount();
      updateConnectionDependentForms();
      showNotification('Unsubscribed from active subscriptions stream.', 'info');
      Logger.info('[Presence] Unsubscribed from active subscriptions stream.');
    });

    // Retry field increment/decrement buttons
    if (DOM.retryInput && DOM.retryDecrement && DOM.retryIncrement) {
      const min = 1000;
      const max = 30000;
      const step = 500;
      DOM.retryDecrement.addEventListener('click', () => {
        const current = parseInt(DOM.retryInput.value, 10) || min;
        DOM.retryInput.value = Math.max(min, current - step);
      });
      DOM.retryIncrement.addEventListener('click', () => {
        const current = parseInt(DOM.retryInput.value, 10) || min;
        DOM.retryInput.value = Math.min(max, current + step);
      });
      // Clamp typed/pasted values to min/max range, strip non-digits
      DOM.retryInput.addEventListener('change', () => {
        const cleaned = DOM.retryInput.value.replace(/\D/g, '');
        const value = parseInt(cleaned, 10);
        if (Number.isNaN(value) || cleaned === '') {
          DOM.retryInput.value = '';
        } else if (value < min) {
          DOM.retryInput.value = min;
        } else if (value > max) {
          DOM.retryInput.value = max;
        } else {
          DOM.retryInput.value = value;
        }
      });
    }
  }

  /** Discovers the server version from the Server HTTP header and injects it into the UI. */
  async function injectVersion() {
    const pill = document.querySelector('.navbar-context-label');
    const footer = document.getElementById('app-version');
    let server;
    try {
      const resp = await fetch(window.location.pathname, { method: 'HEAD' });
      if (!resp.ok) Logger.debug(`[Version] HEAD request returned ${resp.status}, continuing.`);
      server = resp.headers.get('Server');
    } catch (err) {
      Logger.debug('[Version] Could not discover server version.', err);
    }
    if (server) {
      if (pill) pill.dataset.tooltip = server;
      if (footer) footer.textContent = server;
    }
    // Ensure the UI always shows something, even if the fetch failed or returned no header.
    if (footer && !footer.textContent) footer.textContent = 'Mercure';
  }

  /**
   * Initializes the application by setting defaults and attaching listeners.
   */
  async function init() {
    Logger.debug('[Init] Application starting.');
    try {
      initializeEventListeners();
      injectVersion(); // Fire-and-forget; version display is non-critical.
      await setDefaultValues();
      updateJwtPayloadDisplay();
      updateAlgorithmSections();

      // Clear any stale auth cookie from a previous session.
      // The cookie is HttpOnly so JS can't detect it, but it persists across reloads
      // and would silently authenticate requests even though the UI defaults to Header auth.
      try {
        const topicUrl = DOM.discoverForm.topic.value;
        if (topicUrl) {
          const resp = await fetch(topicUrl, { credentials: 'include' });
          if (resp.ok) Logger.debug('[Init] Cleared stale auth cookie.');
          else Logger.debug('[Init] Stale cookie cleanup got non-OK response:', resp.status);
        }
      } catch (err) {
        Logger.debug('[Init] Could not clear stale auth cookie (non-critical).', err);
      }

      const loadingScreen = document.getElementById('loading-screen');
      if (loadingScreen) {
        loadingScreen.classList.add('is-hidden');
      } else {
        Logger.warn('[Init] Loading screen element not found.');
      }
      Logger.info('[Init] Application ready.');
    } catch (err) {
      Logger.error('[Init] Application failed to initialize.', err);
      showNotification(
        `Application failed to load: ${formatErrorMessage(err)}. Please refresh the page.`,
        'error',
        10000,
      );
      const loadingScreen = document.getElementById('loading-screen');
      if (loadingScreen) loadingScreen.classList.add('is-hidden');
    }
  }

  // Start the application once the DOM is fully loaded.
  document.addEventListener('DOMContentLoaded', init);
})();
