<script>
export default {
  name: "ConversationsView",

  data() {
    return {
      loading: true,
      errormsg: null,
      items: [],
      userPhotoFile: null,


      // Dialog "Nuova conversazione"
      newDlg: {
        open: false,
        type: "dm", // "dm" | "group"
        submitting: false,
        // 1–1
        dm: {
          query: "",
          results: [],
          selected: null,
          text: "👋", // primo messaggio richiesto da /v1/messages
          _timer: null,
        },
        // gruppo
        group: {
          name: "",
          query: "",
          results: [],
          members: [],
          _timer: null,
        },
      },
    };
    
  },

  async mounted() {
    await this.refresh();
  },

  beforeUnmount() {
    // evita timer appesi dall'autocomplete
    clearTimeout(this.newDlg?.dm?._timer);
    clearTimeout(this.newDlg?.group?._timer);
  },

  methods: {
    // ===== list =====
    async refresh() {
      this.loading = true;
      this.errormsg = null;
      try {
        const { data } = await this.$axios.get("/v1/me/conversations");
        this.items = [...data].sort(
          (a, b) => new Date(b.lastMessageAt || 0) - new Date(a.lastMessageAt || 0)
        );
      } catch (e) {
        this.errormsg = e?.response?.data?.message || e?.message || "Errore";
      } finally {
        this.loading = false;
      }
    },
    async uploadMyPhoto(){
      if (!this.userPhotoFile) return;
      const fd = new FormData();
      fd.append("photo", this.userPhotoFile);
      await this.$axios.put("/v1/me/photo", fd, { headers:{ "Content-Type":"multipart/form-data" }});
      this.userPhotoFile = null;
      await this.refresh(); // ricarica la lista per vedere l’avatar aggiornato
    },

    openConversation(id) {
      // usa la route nominata che mappa /chat/:id
      this.$router.push({ name: "conversation", params: { id } });
    },

    // ===== dialog =====
    openNew() {
      this.newDlg.open = true;
      this.newDlg.type = "dm";
      this.errormsg = null;
      // reset campi
      this.newDlg.dm = { query: "", results: [], selected: null, text: "👋", _timer: null };
      this.newDlg.group = { name: "", query: "", results: [], members: [], _timer: null };
    },
    closeNew() {
      this.newDlg.open = false;
    },

    // ---- ricerca utenti (debounced) ----
    searchUsers(which) {
      const box = which === "dm" ? this.newDlg.dm : this.newDlg.group;
      clearTimeout(box._timer);
      box._timer = setTimeout(async () => {
        const q = (box.query || "").trim();
        if (!q) {
          box.results = [];
          return;
        }
        try {
          const { data } = await this.$axios.get("/v1/users", { params: { q } }); // [{id,name}]
          box.results = data;
        } catch {
          /* silenzio, è solo un autocomplete */
        }
      }, 250);
    },

    chooseDmTarget(u) {
      this.newDlg.dm.selected = u;
      this.newDlg.dm.query = u.name;
      this.newDlg.dm.results = [];
    },

    addGroupMember(u) {
      if (!this.newDlg.group.members.find((m) => m.id === u.id)) {
        this.newDlg.group.members.push(u);
      }
      this.newDlg.group.query = "";
      this.newDlg.group.results = [];
    },

    removeGroupMember(u) {
      this.newDlg.group.members = this.newDlg.group.members.filter((m) => m.id !== u.id);
    },

    async leaveGroup(id) {
      if (!confirm("Leave this group?")) return;
      try {
        await this.$axios.delete(`/v1/groups/${id}/members`);
        await this.refresh(); // ricarica la lista
      } catch (e) {
        this.errormsg = e?.response?.data?.message || e?.message || "Errore";
      }
    },

    // ---- submit ----
    async submitNew() {
      this.errormsg = null;
      this.newDlg.submitting = true;
      try {
        if (this.newDlg.type === "dm") {
          let sel = this.newDlg.dm.selected;

          // fallback: se non ha cliccato il suggerimento, prova match esatto sull'username
          if (!sel && this.newDlg.dm.query.trim()) {
            const q = this.newDlg.dm.query.trim();
            const { data: users } = await this.$axios.get("/v1/users", { params: { q } });
            sel = users.find((u) => u.name === q);
          }

          if (!sel) {
            this.errormsg = "Seleziona un destinatario";
            return;
          }

          const text = (this.newDlg.dm.text || "👋").trim();
          const { data } = await this.$axios.post("/v1/messages", {
            toUserId: sel.id,
            text,
          });
          this.closeNew();
          await this.refresh();
          this.$router.push({ name: "conversation", params: { id: data.conversationId } });
          return;
        }

        // gruppo
        const name = (this.newDlg.group.name || "").trim();
        if (!name) {
          this.errormsg = "Inserisci un nome per il gruppo";
          return;
        }

        const { data } = await this.$axios.post("/v1/conversations", { name, isGroup: true });
        const groupId = data.conversationId;

        // aggiungi membri selezionati
        for (const m of this.newDlg.group.members) {
          await this.$axios.post(`/v1/groups/${groupId}/members`, { userId: m.id });
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
  <div class="container py-5" style="max-width:720px">
    <div class="d-flex align-items-center gap-2">
      <label class="btn btn-outline-dark btn-sm mb-0">
        Upload me
        <input type="file" accept="image/*" class="d-none" @change="e => userPhotoFile = e.target.files[0]" />
      </label>
      <button class="btn btn-dark btn-sm" @click="uploadMyPhoto" :disabled="!userPhotoFile">Carica</button>
      <button class="btn btn-dark btn-sm" @click="openNew">+ New</button>
    </div>


    <p v-if="loading">Loading…</p>
    <p v-if="errormsg" class="text-danger">{{ errormsg }}</p>

    <div v-if="!loading && !errormsg && items.length === 0" class="text-muted">
      No conversations yet
    </div>

    <ul v-if="!loading && !errormsg && items.length" class="list-unstyled">
      <li v-for="c in items" :key="c.id" class="mb-3">
        <div class="card-mono p-3" role="button" @click="openConversation(c.id)">
          <div class="d-flex justify-content-between align-items-center">
            <strong class="text-truncate">{{ c.name }}</strong>
            <div class="d-flex align-items-center gap-2">
              <!-- stop evita l'apertura della chat -->
              <button
                v-if="c.isGroup"
                type="button"
                class="btn btn-sm btn-outline-danger"
                @click.stop="leaveGroup(c.id)"
              >
                Leave group
              </button>
            </div>
          </div>
          <div class="text-muted mt-1 text-truncate">
            {{ c.lastMessageText || "…" }}
          </div>
        </div>
      </li>
    </ul>

    <!-- Dialog semplice -->
    <div v-if="newDlg.open" class="dlg-backdrop">
      <div class="dlg card-mono p-3">
        <div class="d-flex justify-content-between align-items-center mb-2">
          <strong>New conversation</strong>
          <button class="btn btn-sm btn-outline-dark" @click="closeNew">Close</button>
        </div>

        <!-- scelta tipo -->
        <div class="mb-3">
          <label class="me-3">
            <input type="radio" value="dm" v-model="newDlg.type" />
            1–1
          </label>
          <label>
            <input type="radio" value="group" v-model="newDlg.type" />
            Group
          </label>
        </div>

        <!-- 1–1 -->
        <div v-if="newDlg.type === 'dm'">
          <label class="form-label">Recipient</label>
          <input
            class="form-control mb-2"
            placeholder="Username…"
            v-model="newDlg.dm.query"
            @input="searchUsers('dm')"
            autocomplete="off"
          />
          <ul class="list-group mb-2" v-if="newDlg.dm.results.length">
            <li
              v-for="u in newDlg.dm.results"
              :key="u.id"
              class="list-group-item list-group-item-action"
              @click="chooseDmTarget(u)"
            >
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

        <!-- Group -->
        <div v-else>
          <label class="form-label">Group name</label>
          <input class="form-control mb-3" v-model="newDlg.group.name" placeholder="Study group" />

          <label class="form-label">Add members</label>
          <input
            class="form-control mb-2"
            placeholder="Search username…"
            v-model="newDlg.group.query"
            @input="searchUsers('group')"
            autocomplete="off"
          />
          <ul class="list-group mb-2" v-if="newDlg.group.results.length">
            <li
              v-for="u in newDlg.group.results"
              :key="u.id"
              class="list-group-item list-group-item-action"
              @click="addGroupMember(u)"
            >
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
  </div>
</template>

<style>
/* overlay ez */
.dlg-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.15);
  display: grid;
  place-items: center;
  z-index: 1000;
}
.dlg {
  width: 520px;
  max-width: 95vw;
}
.card-mono {
  border: 2px solid #111;
  border-radius: 8px;
  background: #fff;
}
.h-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono",
    monospace;
  letter-spacing: 0.08em;
}
</style>
