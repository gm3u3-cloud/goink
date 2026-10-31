/**
 * Returns the matching i18n key for a provider error message.
 * Components use t(explainErrorKey(msg)) to display the translated
 * explanation.
 *
 * Matching rules: map common error keywords (status codes, message content)
 * to i18n keys.
 * Covers mainstream providers (OpenAI/Anthropic/Gemini/DeepSeek/Kimi/GLM/
 * Qwen/MiMo, etc.) and relay stations.
 *
 * i18n key namespace: settings.errorHints.*
 *
 * Note: the CJK keywords inside the regexes (余额不足, 限流, 超时, ...) are
 * data values matched against Chinese provider error messages — they must
 * not be translated (plan rule §1).
 */
export function explainErrorKey(msg: string): string {
  if (!msg) {
    return "settings.errorHints.generic";
  }

  const lower = msg.toLowerCase();

  if (
    /401|invalid.*key|invalid_key|authentication.*error|api[_-]?key.*invalid/.test(
      lower,
    )
  ) {
    return "settings.errorHints.invalidKey";
  }
  if (
    /402|余额不足|quota|insufficient.*balance|insufficient_quota|credit/.test(
      lower,
    )
  ) {
    return "settings.errorHints.insufficientBalance";
  }
  if (/403|forbidden|权限|access.*denied/.test(lower)) {
    return "settings.errorHints.forbidden";
  }
  if (/404|html 错误页|not\s*found/.test(lower)) {
    return "settings.errorHints.notFound";
  }
  if (/429|rate.*limit|too.*many|限流/.test(lower)) {
    return "settings.errorHints.rateLimit";
  }
  if (
    /50[023]|internal.*server|bad.*gateway|service.*unavailable/.test(lower)
  ) {
    return "settings.errorHints.serverError";
  }
  if (/sse|流.*未找到|chunk|非标准|choices/.test(lower)) {
    return "settings.errorHints.nonStandardSSE";
  }
  if (/timeout|超时|context.*deadline|deadline.*exceeded/.test(lower)) {
    return "settings.errorHints.timeout";
  }
  if (/url.*空|url 为空/.test(lower)) {
    return "settings.errorHints.emptyUrl";
  }

  return "settings.errorHints.generic";
}
