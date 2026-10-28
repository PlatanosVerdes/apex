export class Api {
  plan(body, signal) {
    return this.request('api/plan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal,
    });
  }

  place({ lat, lng }) {
    return this.request(`api/place?lat=${lat.toFixed(6)}&lng=${lng.toFixed(6)}`);
  }

  routes() {
    return this.request('api/routes');
  }

  save(route) {
    const { id, ...body } = route;
    return this.request(id ? `api/routes/${id}` : 'api/routes', {
      method: id ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  }

  search(q, near, signal) {
    const at = near ? `&near=${near.lat.toFixed(4)},${near.lng.toFixed(4)}` : '';
    return this.request(`api/search?q=${encodeURIComponent(q)}${at}`, { signal });
  }

  me() {
    return this.request('api/me');
  }

  logout() {
    return this.request('auth/logout', { method: 'POST' });
  }

  lists() {
    return this.request('api/lists');
  }

  addList(url) {
    return this.request('api/lists', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    });
  }

  readList(url) {
    return this.request('api/lists/read', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url }),
    });
  }

  refreshList(id) {
    return this.request(`api/lists/${encodeURIComponent(id)}/refresh`, { method: 'POST' });
  }

  setListColour(id, colour) {
    return this.request(`api/lists/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ colour }),
    });
  }

  removeList(id) {
    return this.request(`api/lists/${encodeURIComponent(id)}`, { method: 'DELETE' });
  }

  remove(id) {
    return this.request(`api/routes/${id}`, { method: 'DELETE' });
  }

  async request(url, init) {
    const res = await fetch(url, init);
    if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
    return res.status === 204 ? null : res.json();
  }
}
