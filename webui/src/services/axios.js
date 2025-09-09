import axios from "axios";

const TOKEN_KEY = "identifier";

const api = axios.create({
  baseURL: __API_URL__,  
  timeout: 10000,
});

api.interceptors.request.use((cfg) => {
  const id = localStorage.getItem(TOKEN_KEY);
  if (id) cfg.headers.Authorization = `Bearer ${id}`;
  return cfg;
});

export function setAuth(identifier) {
  if (identifier) localStorage.setItem(TOKEN_KEY, identifier);
  else localStorage.removeItem(TOKEN_KEY);
}

export default api;
