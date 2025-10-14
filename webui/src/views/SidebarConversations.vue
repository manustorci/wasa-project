<script>
export default {
  name: "SidebarConversations",
  data() {
    return {
      items: [],
      loading: true,
      errormsg: null,

      // dialog "nuova conversazione"
      newDlg: {
        open: false,
        type: "dm",                 // "dm" | "group"
        submitting: false,
        dm: { query: "", results: [], selected: null, text: "👋", _timer: null },
        group: { name: "", query: "", results: [], members: [], _timer: null },
      },
    };
  },
  async mounted() { await this.refresh(); },

  methods: {
    photoSrc(p){
      if (!p) return null;
      // se già assoluto
      if (p.startsWith("http://") || p.startsWith("https://")) return p;
      // altrimenti relativo alla baseURL dell’axios
      const base = this.$axios?.defaults?.baseURL?.replace(/\/+$/,"") || "";
      return `${base}/${p.replace(/^\/+/,"")}`;
    },

    async refresh() {
      this.loading = true; this.errormsg = null;
      try {
        const { data } = await this.$axios.get("/me/conversations");
        this.items = [...data].sort(
          (a, b) => new Date(b.lastMessageAt || 0) - new Date(a.lastMessageAt || 0)
        );
      } catch (e) {
        this.errormsg = e?.response?.data?.message || e?.message || "Errore";
      } finally { this.loading = false; }
    },

    // --- dialog ---
    openNew() {
      this.newDlg.open = true;
      this.newDlg.type = "dm";
      this.errormsg = null;
      this.newDlg.dm    = { query:"", results:[], selected:null, text:"👋", _timer:null };
      this.newDlg.group = { name:"", query:"", results:[], members:[], _timer:null };
    },
    closeNew(){ this.newDlg.open = false; },

    searchUsers(which){
      const box = which === "dm" ? this.newDlg.dm : this.newDlg.group;
      clearTimeout(box._timer);
      box._timer = setTimeout(async () => {
        const q = (box.query || "").trim();
        if (!q) { box.results = []; return; }
        try {
          const { data } = await this.$axios.get("/users", { params:{ q } });
          box.results = data; // [{id, name}]
        } catch {}
      }, 250);
    },
    chooseDmTarget(u){ this.newDlg.dm.selected = u; this.newDlg.dm.query = u.name; this.newDlg.dm.results = []; },
    addGroupMember(u){
      if (!this.newDlg.group.members.find(m => m.id === u.id)) {
        this.newDlg.group.members.push(u);
      }
      this.newDlg.group.query = ""; this.newDlg.group.results = [];
    },
    removeGroupMember(u){
      this.newDlg.group.members = this.newDlg.group.members.filter(m => m.id !== u.id);
    },

    async submitNew(){
      this.errormsg = null; this.newDlg.submitting = true;
      try {
        if (this.newDlg.type === "dm") {
          let sel = this.newDlg.dm.selected;
          if (!sel && this.newDlg.dm.query.trim()) {
            const q = this.newDlg.dm.query.trim();
            const { data: users } = await this.$axios.get("/users", { params:{ q } });
            sel = users.find(u => u.name === q);  // match esatto username
          }
          if (!sel) { this.errormsg = "Seleziona un destinatario"; return; }

          const text = (this.newDlg.dm.text || "👋").trim();
          const { data } = await this.$axios.post("/messages", { toUserId: sel.id, text });
          this.closeNew();
          await this.refresh();
          this.$router.push({ name: "conversation", params: { id: data.conversationId } });
          return;
        }

        // group
        const name = (this.newDlg.group.name || "").trim();
        if (!name) { this.errormsg = "Inserisci un nome per il gruppo"; return; }

        const { data } = await this.$axios.post("/conversations", { name, isGroup: true });
        const groupId = data.conversationId;
        for (const m of this.newDlg.group.members) {
          await this.$axios.post(`/groups/${groupId}/members`, { userId: m.id });
        }
        this.closeNew();
        await this.refresh();
        this.$router.push({ name: "conversation", params: { id: groupId } });
      } catch (e) {
        this.errormsg = e?.response?.data?.message || e?.message || "Errore creazione";
      } finally {
        this.newDlg.submitting = false;
      }
    },
  },
};
</script>

<template>
  <!-- HEADER -->
  <div class="d-flex align-items-center justify-content-between p-3 border-bottom">
    <h2 class="h-mono fs-4 mb-0">Conversations</h2>
    <button class="btn btn-dark btn-sm" @click="openNew">+ New</button>
  </div>

  <!-- LISTA -->
  <ul class="list-unstyled m-0 p-3">
    <li v-for="c in items" :key="c.id" class="mb-3">
      <router-link
        :to="{ name: 'conversation', params: { id: c.id } }"
        class="card-mono p-3 d-block text-reset text-decoration-none conv-item d-flex align-items-center"
        :class="{ 'is-active': String($route.params.id||'') === String(c.id) }"
      >
        <div class="conv-avatar me-3">
          <img v-if="photoSrc(c.photoUrl)" :src="photoSrc(c.photoUrl)" alt="" />
          <div v-else class="conv-fallback">{{ (c.name||'?').slice(0,1).toUpperCase() }}</div>
        </div>

        <div class="flex-grow-1">
          <div class="d-flex justify-content-between align-items-center">
            <strong class="text-truncate">{{ c.name }}</strong>
          </div>
          <div class="text-muted mt-1 text-truncate">
            {{ c.lastMessageText || "…" }}
          </div>
        </div>
      </router-link>
    </li>
  </ul>

  <!-- DIALOG nuova conversazione -->
  <div v-if="newDlg.open" class="dlg-backdrop">
    <div class="dlg card-mono p-3">
      <div class="d-flex justify-content-between align-items-center mb-2">
        <strong>New conversation</strong>
        <button class="btn btn-sm btn-outline-dark" @click="closeNew">Close</button>
      </div>

      <div class="mb-3">
        <label class="me-3"><input type="radio" value="dm" v-model="newDlg.type"> 1–1</label>
        <label><input type="radio" value="group" v-model="newDlg.type"> Group</label>
      </div>

      <!-- DM -->
      <div v-if="newDlg.type==='dm'">
        <label class="form-label">Recipient</label>
        <input class="form-control mb-2" placeholder="Username…" v-model="newDlg.dm.query"
               @input="searchUsers('dm')" autocomplete="off" />
        <ul class="list-group mb-2" v-if="newDlg.dm.results.length">
          <li v-for="u in newDlg.dm.results" :key="u.id"
              class="list-group-item list-group-item-action"
              @click="chooseDmTarget(u)">
            {{ u.name }}
          </li>
        </ul>
        <div class="mb-2" v-if="newDlg.dm.selected">
          Selected: <code>{{ newDlg.dm.selected.name }}</code>
        </div>

        <label class="form-label">First message</label>
        <input class="form-control mb-3" v-model="newDlg.dm.text" />

        <button class="btn btn-dark w-100" :disabled="newDlg.submitting" @click="submitNew">
          Start chat
        </button>
      </div>

      <!-- GROUP -->
      <div v-else>
        <label class="form-label">Group name</label>
        <input class="form-control mb-3" v-model="newDlg.group.name" placeholder="Study group" />

        <label class="form-label">Add members</label>
        <input class="form-control mb-2" placeholder="Search username…" v-model="newDlg.group.query"
               @input="searchUsers('group')" autocomplete="off" />
        <ul class="list-group mb-2" v-if="newDlg.group.results.length">
          <li v-for="u in newDlg.group.results" :key="u.id"
              class="list-group-item list-group-item-action"
              @click="addGroupMember(u)">
            {{ u.name }}
          </li>
        </ul>

        <div class="mb-3" v-if="newDlg.group.members.length">
          <span v-for="m in newDlg.group.members" :key="m.id" class="badge bg-dark me-2">
            {{ m.name }}
            <span role="button" class="ms-1" @click="removeGroupMember(m)">×</span>
          </span>
        </div>

        <button class="btn btn-dark w-100" :disabled="newDlg.submitting" @click="submitNew">
          Create group
        </button>
      </div>

      <p v-if="errormsg" class="text-danger mt-3 mb-0">{{ errormsg }}</p>
    </div>
  </div>
</template>

<style scoped>

.conv-avatar{
  width:40px; height:40px; border:2px solid #111; border-radius:50%;
  display:grid; place-items:center; overflow:hidden; background:#fff; flex:0 0 40px;
}
.conv-avatar img{ width:100%; height:100%; object-fit:cover; display:block; }
.conv-fallback{ font-weight:700; }

.card-mono{ border:2px solid #111; border-radius:8px; background:#fff; }
.h-mono{ font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,"Liberation Mono",monospace; letter-spacing:.08em; }
.conv-item.is-active{ box-shadow: inset 0 0 0 3px #111; background:#f5f5f5; }

/* dialog */
.dlg-backdrop{ position:fixed; inset:0; background:rgba(0,0,0,.15); display:grid; place-items:center; z-index:1000; }
.dlg{ width:520px; max-width:95vw; }
</style>

