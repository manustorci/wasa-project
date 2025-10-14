<script>
export default {
  name: "ConversationView",
  data() {
    return {
      convo: null,
      groupPhotoFile: null,
      text: "",
      errormsg: null,
      loading: false,
      refreshTimer: null,
      newUser:"",
      title: "",
      isGroup: false,
      menuOpen: false,
      newName: "",
      modalType: null,
      emojiSet: ["👍","❤️","😂","🎉","😮","😢","🔥","🙏"],
      pickerForId: null, // id del mex che ha il picker aperto
      msgMenuFor: null,

      forwardDlg: {
        open: false,
        msg: null,         
        items: [],         
        loading: false,
        submitting: false,
        targetId: null,     
        query: "",         
        err: null,
      },
    };
  },

  async mounted() {
    await this.load();
    this.refreshTimer = setInterval(async () => { await this.load() }, 5000);
    this.loadHeadInfo();
  },

  beforeUnmount() {
    clearInterval(this.refreshTimer);
  },

  watch: {
    '$route.params.id': {
      handler() {
        this.load();
        this.loadHeadInfo(); 
      }
    }
  },

  methods: {

    async changeGroupPhoto(){
      if (!this.groupPhotoFile) return;
      const id = this.$route.params.id;
      const fd = new FormData();
      fd.append("photo", this.groupPhotoFile);
      await this.$axios.put(`/groups/${id}/photo`, fd, { headers:{ "Content-Type":"multipart/form-data" }});
      this.groupPhotoFile = null;
      await this.loadHeadInfo(); // per aggiornare eventualmente name/photo lato lista
    },

    // --- reactions utils ---
    msgComments(m) {
      return Array.isArray(m.comments) ? m.comments : [];
    },
    hasReacted(m, emoji) {
      const me = localStorage.getItem("identifier");
      return this.msgComments(m).some(
        c => c.comment === emoji && (c.userId === me || c.user === me)
      );
    },

    toggleMsgMenu(m) {
    const key = m.id ?? m.timestamp;
    this.msgMenuFor = (this.msgMenuFor === key) ? null : key;
    },

    async deleteMessage(m) {
      if (!confirm("Annullare questo messaggio?")) return;
      try {
        await this.$axios.delete(`/messages/${m.id}`);
        // ottimistico: rimuovo localmente senza ricaricare tutto
        this.convo.messages = (this.convo.messages || []).filter(x => x.id !== m.id);
        this.msgMenuFor = null;
      } catch (e) {
        alert(e?.response?.data?.message || e.message || "Cancellation error");
      }
    },

    openForward(m) {
      this.forwardDlg.open = true;
      this.forwardDlg.msg = m;        
      this.forwardDlg.items = [];
      this.forwardDlg.targetId = null;
      this.forwardDlg.query = "";
      this.forwardDlg.err = null;
      this.loadForwardTargets();
    },

    closeForward() {
      this.forwardDlg.open = false;
      this.forwardDlg.msg = null;
    },

    async loadForwardTargets() {
      this.forwardDlg.loading = true; this.forwardDlg.err = null;
      try {
        const { data } = await this.$axios.get("/me/conversations");
        const currentId = Number(this.$route.params.id);
        this.forwardDlg.items = (data || []).filter(c => c.id !== currentId);
      } catch (e) {
        this.forwardDlg.err = e?.response?.data?.message || e?.message || "Loading error";
      } finally {
        this.forwardDlg.loading = false;
      }
    },

    async doForward() {
      const msg = this.forwardDlg.msg;
      const dst = this.forwardDlg.targetId;
      if (!msg?.id || !dst) return;

      this.forwardDlg.submitting = true; this.forwardDlg.err = null;
      try {
        await this.$axios.post(`/messages/${msg.id}/forward`, { conversationId: dst });
        this.closeForward();

      } catch (e) {
        this.forwardDlg.err = e?.response?.data?.message || e?.message || "Forward error";
      } finally {
        this.forwardDlg.submitting = false;
      }
    },


    groupByEmoji(m) {
      const map = {};
      for (const c of this.msgComments(m)) map[c.comment] = (map[c.comment] || 0) + 1;
      return map; // es. { "👍": 2, "❤️": 1 }
    },
    async toggleReaction(m, emoji) {
      try {
        if (this.hasReacted(m, emoji)) {
          await this.$axios.delete(`/messages/${m.id}/comments`);
        } else {
          await this.$axios.post(`/messages/${m.id}/comments`, { comment: emoji });
        }
        this.pickerForId = null;
        await this.load();
      } catch (e) {
        alert(e?.response?.data?.message || e.message || "Error reaction");
      }
    },
    openPicker(m) {
      // usa id se c'è, altrimenti timestamp come fallback
      this.pickerForId = m.id ?? m.timestamp;
    },
    closePicker() { this.pickerForId = null; },

    // --- load & send ---
    async load() {
      this.loading = true; this.errormsg = null;
      try {
        const { id } = this.$route.params;
        const { data } = await this.$axios.get(`/conversations/${id}`);
        this.convo = data;
      } catch (e) {
        this.errormsg = e?.response?.data?.message || e?.message || "Error";
      } finally {
        this.loading = false;
      }
    },
    async send() {
      if (!this.text.trim()) return;
      try {
        const { id } = this.$route.params;
        await this.$axios.post(`/conversations/${id}/messages`, { text: this.text.trim() });
        this.text = "";
        await this.load();
      } catch (e) {
        this.errormsg = e?.response?.data?.message || e?.message || "Failed sent";
      }
    },

    // --- header info ---
    async loadHeadInfo() {
      const id = Number(this.$route.params.id);
      try {
        const { data } = await this.$axios.get("/me/conversations");
        const conv = data.find(c => c.id === id);
        if (conv) { this.title = conv.name; this.isGroup = !!conv.isGroup; }
        else { this.title = ""; this.isGroup = false; }
      } catch {
        this.title = ""; this.isGroup = false;
      }
    },

    // --- settings modal ---
    openRename() { this.menuOpen = false; this.modalType = "rename"; this.newName = this.title; },
    openAddUser() { this.menuOpen = false; this.modalType = "addUser"; this.newUser = ""; },
    closeModal() { this.modalType = null; },

    async doRename() {
      const id = this.$route.params.id;
      if (!this.newName?.trim()) return;
      await this.$axios.put(`/groups/${id}/name`, { name: this.newName.trim() });
      this.title = this.newName.trim();
      this.modalType = null;
    },

    async doAddUser() {
      const query = this.newUser?.trim();
      if (!query) return;
      const id = this.$route.params.id;
      try {
        const { data: users } = await this.$axios.get("/users", { params: { q: query } });
        const user = users.find(u => u.name === query) || users[0];
        if (!user) { alert("Utente non trovato"); return; }
        await this.$axios.post(`/groups/${id}/members`, { userId: user.id });
        this.newUser = "";
        await this.loadHeadInfo(); await this.load();
        this.modalType = null;
      } catch (e) {
        alert(e?.response?.data?.message || e.message || "Error");
      }
    },

    confirmLeave() {
      this.menuOpen = false;
      if (!confirm("Are you sure to leave the group?")) return;
      this.leaveGroup();
    },
    async leaveGroup() {
      const id = this.$route.params.id;
      await this.$axios.delete(`/groups/${id}/members`);
      this.$router.push("/chat");
    },

    formatDate(ts) {
      const d = new Date(ts);
      return d.toLocaleString("it-IT",{ day:"2-digit", month:"2-digit", hour:"2-digit", minute:"2-digit" });
    },
  }


};
</script>


<template>
  <div class="container py-3" v-if="convo">
    <!-- HEADER: titolo + menu impostazioni (in alto, vicino ai tre nomi) -->
    <div class="d-flex align-items-center justify-content-between mb-3 position-relative">
      <h1 class="h5 mb-0">
        {{ isGroup && title ? title : (convo.participants || []).join(", ") }}
      </h1>

      <div class="position-relative">
        <button class="btn-mono btn-mono--sm" @click="menuOpen = !menuOpen">
          Settings
        </button>

        <div v-if="menuOpen" class="position-absolute end-0 mt-2 menu-mono" style="z-index:10;">
          <button class="link-mono" :disabled="!isGroup" @click="openRename()">Change group name</button>
          <button class="link-mono" :disabled="!isGroup" @click="openAddUser()">Add user</button>
           <button class="link-mono" :disabled="!isGroup" @click="$refs.grpPhoto.click()">Change group photo</button>
          <input
            ref="grpPhoto"
            type="file"
            accept="image/*"
            class="d-none"
            @change="e => { groupPhotoFile = e.target.files[0]; changeGroupPhoto(); }"
          />
          <button class="link-mono link-mono--danger" :disabled="!isGroup" @click="confirmLeave()">Leave group</button>

        </div>
      </div>
    </div>


    <!-- lista messaggi -->
    <ul class="list-group mb-3">      
      <li
        v-for="m in (convo.messages || [])"
        :key="m.id ?? m.timestamp"
        class="list-group-item"
      >

      <!-- TESTO MESSAGGIO (sinistra) + ORA (destra) -->
      <div class="d-flex justify-content-between align-items-start">
        <span><b>{{ m.sender }}</b> — {{ m.text }}</span>
        <div class="d-flex align-items-center">
          <small class="text-muted ms-2">{{ formatDate(m.timestamp) }}</small>

          <!-- bottone menu ⋯ (sempre visibile) -->
          <div class="position-relative ms-2">
            <button type="button" class="btn-icon" @click="toggleMsgMenu(m)">⋯</button>
            <div
              v-if="msgMenuFor === (m.id ?? m.timestamp)"
              class="dropdown-card"
            >
              <button class="dropdown-item text-danger" @click="deleteMessage(m)">
                Delete
              </button>
              <button class="dropdown-item" @click="openForward(m)">
                Forward
              </button>
            </div>
          </div>
        </div>
      </div>


        <!-- REACTIONS -->
        <div class="mt-2 d-flex align-items-center flex-wrap gap-2 position-relative">
          <template v-for="(count, emoji) in groupByEmoji(m)" :key="emoji">
            <button
              type="button"
              class="rxn-pill"
              :class="{ 'is-mine': hasReacted(m, emoji) }"
              @click="toggleReaction(m, emoji)"
            >
              {{ emoji }} <span v-if="count">{{ count }}</span>
            </button>
          </template>

          <button type="button" class="rxn-add" @click="openPicker(m)">+</button>

          <div v-if="pickerForId === (m.id ?? m.timestamp)" class="rxn-picker">
            <button v-for="e in emojiSet" :key="e" type="button" class="rxn-item" @click="toggleReaction(m, e)">{{ e }}</button>
            <button type="button" class="rxn-close" @click="closePicker">×</button>
          </div>
        </div>
      </li>
    </ul>

    <!-- composer -->
    <form class="d-flex gap-2" @submit.prevent="send">
      <input v-model="text" class="form-control" placeholder="Write a message…" required />
      <button class="btn btn-dark">Send</button>
    </form>

    <p v-if="errormsg" class="text-danger mt-2">{{ errormsg }}</p>
  </div>

  <p v-else class="container py-3">Loading…</p>

    <!-- unico modal -->
    <div v-if="modalType" class="dlg-backdrop" @click.self="closeModal">
      <div class="dlg card-mono p-3">
        <div class="d-flex justify-content-between align-items-center mb-2">
          <strong>{{ modalType === 'rename' ? 'Change group name' : 'Add user' }}</strong>
          <button class="btn btn-sm btn-outline-dark" @click="closeModal">Close</button>
        </div>

        <!-- Rinomina -->
        <template v-if="modalType === 'rename'">
          <input
            v-model="newName"
            type="text"
            maxlength="50"
            class="form-control input-mono mb-3"
            placeholder="New name…"
          />
          <button class="btn-mono" @click="doRename">Save</button>
        </template>

        <!-- Aggiungi utente -->
        <template v-else>
          <label class="form-label">Username</label>
          <input
            v-model="newUser"
            type="text"
            class="form-control input-mono mb-3"
            placeholder="Username…"
          />
          <button class="btn-mono" @click="doAddUser">Add</button>
        </template>
      </div>
    </div> 


    <!-- DIALOG INOLTRA -->
    <div v-if="forwardDlg.open" class="dlg-backdrop" @click.self="closeForward">
      <div class="dlg card-mono p-3">
        <div class="d-flex justify-content-between align-items-center mb-2">
          <strong>Forward</strong>
          <button class="btn btn-sm btn-outline-dark" @click="closeForward">Close</button>
        </div>

        <div class="mb-2">
          <input
            class="form-control"
            placeholder="Filter conversation.."
            v-model="forwardDlg.query"
            autocomplete="off"
          />
        </div>

        <div v-if="forwardDlg.loading" class="text-muted">Loading..</div>
        <div v-else>
          <ul class="list-group mb-2" style="max-height: 300px; overflow:auto;">
            <li
              v-for="c in forwardDlg.items.filter(
                    it => !forwardDlg.query ||
                          (it.name || '').toLowerCase().includes(forwardDlg.query.toLowerCase())
                  )"
              :key="c.id"
              class="list-group-item d-flex align-items-center"
              @click="forwardDlg.targetId = c.id"
              :class="{ active: forwardDlg.targetId === c.id }"
              style="cursor:pointer"
            >
              <input type="radio" class="form-check-input me-2" :checked="forwardDlg.targetId === c.id" />
              <span class="text-truncate">{{ c.name }}</span>
            </li>
          </ul>
          <div v-if="!forwardDlg.items.length" class="text-muted">No conversation avaible</div>
        </div>

        <p v-if="forwardDlg.err" class="text-danger mb-2">{{ forwardDlg.err }}</p>

        <button
          class="btn btn-dark w-100"
          :disabled="!forwardDlg.targetId || forwardDlg.submitting"
          @click="doForward"
        >
          Forward
        </button>
      </div>
    </div>
</template>



<style scoped>
.dlg-backdrop{ position:fixed; inset:0; background:rgba(0,0,0,.15); display:grid; place-items:center; z-index:1000; }
.dlg{ width:520px; max-width:95vw; }
</style>
