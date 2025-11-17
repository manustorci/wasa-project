<template>
  <div class="me-container">
    <!-- Cerchio ME -->
    <div class="me-dock" @click="pick">
      <img v-if="srcOk" :src="srcOk" alt="me" />
      <span v-else class="fallback">ME</span>
      <input
        ref="picker"
        type="file"
        class="hidden"
        accept="image/png,image/jpeg,image/webp,image/gif"
        @change="onPick"
      />
    </div>

    <!-- Campo cambio username -->
    <div class="me-username">
      <span class="current-username">{{ currentUsername }}</span>
      <input
        v-model="newUsername"
        class="form-control form-control-sm"
        placeholder="New username"
      />
      <button class="btn btn-sm btn-outline-dark" @click="changeUsername">
        Change
      </button>
    </div>
  </div>
</template>

<script>
export default {
  name: "MyAvatarDock",
  data: () => ({
    myUrl: localStorage.getItem("myPhotoUrl") || "",
    origin: "",
    newUsername: "",
    currentUsername: localStorage.getItem("myUsername") || ""
  }),
  computed: {
    srcOk() {
      const p = this.myUrl;
      if (!p) return "";
      if (/^https?:\/\//i.test(p)) return p;
      if (!this.origin) return "";
      const path = p.startsWith("/") ? p : `/${p}`;
      return this.origin + path;
    },
  },
  mounted() {
    try {
      const base = this.$axios?.defaults?.baseURL || "";
      this.origin = base ? new URL(base).origin : window.location.origin;
    } catch {
      this.origin = window.location.origin;
    }
  },
  methods: {
    pick() {
      this.$refs.picker?.click();
    },
    async onPick(e) {
      const f = e.target.files?.[0];
      if (!f) return;
      const fd = new FormData();
      fd.append("photo", f);
      try {
        const res = await this.$axios?.put?.("/me/photo", fd, {
          headers: { "Content-Type": "multipart/form-data" },
        });
        const url = res?.data?.url;
        if (url) {
          localStorage.setItem("myPhotoUrl", url);
          this.myUrl = url + `?t=${Date.now()}`;
        } else {
          window.location.reload();
        }
      } catch (err) {
        console.error(err);
        alert("Upload failed");
      } finally {
        e.target.value = "";
      }
    },
    async changeUsername() {
      const name = this.newUsername.trim();
      if (!name) return alert("Insert a valid username");

      try {
        await this.$axios.put("/me/username", { name });
        alert("Username updated!");
        this.currentUsername = name;
        localStorage.setItem("myUsername", name);
        this.newUsername = "";
      } catch (e) {
        console.error(e);
        alert("Failed to update username");
      }
    },
  },
};
</script>

<style scoped>
.me-container {
  position: fixed;
  left: 16px;
  bottom: 16px;
  display: flex;
  align-items: center;
  gap: 10px;
  z-index: 2000;
}

.me-dock {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  border: 2px solid #111;
  background: #fff;
  overflow: hidden;
  display: grid;
  place-items: center;
  cursor: pointer;
}

.me-dock img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.me-username input {
  width: 130px;
}

.hidden {
  display: none;
}

.fallback {
  font-weight: 800;
}

.current-username {
  font-size: 13px;
  font-weight: 600;
  margin-right: 6px;
}

</style>

