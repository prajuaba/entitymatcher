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
