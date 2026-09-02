export async function apiFetch(path, options = {}) {
  const token = localStorage.getItem("entity_matcher_token");

  const fetchOptions = {
    ...options,
    headers: {
      ...options.headers,
      ...(token && { Authorization: `Bearer ${token}` })
    }
  };

  const response = await fetch(path, fetchOptions);

  if (response.status === 401) {
    localStorage.removeItem("entity_matcher_token");
    dispatchEvent(new CustomEvent("auth:unauthorized"));
    throw new Error("Unauthorized");
  }

  return response;
}

export function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 100);
}

export function getAccessToken() {
  return localStorage.getItem("entity_matcher_token");
}

// The Go backend reports errors with http.Error, which writes a plain-text body.
// Parsing that as JSON throws a SyntaxError that masks the real message, so read
// the body as text first and only then try to interpret it as JSON.
export async function readErrorMessage(response, fallback = "Request failed") {
  let body = "";
  try {
    body = await response.text();
  } catch {
    return `${fallback} (HTTP ${response.status})`;
  }

  const trimmed = body.trim();
  if (!trimmed) return `${fallback} (HTTP ${response.status})`;

  try {
    const parsed = JSON.parse(trimmed);
    return parsed.message || parsed.error || trimmed;
  } catch {
    return trimmed;
  }
}
