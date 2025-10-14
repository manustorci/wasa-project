<template>
  <div class="me-dock" @click="pick">
    <img v-if="srcOk" :src="srcOk" alt="me" />
    <span v-else class="fallback">ME</span>
    <input ref="picker" type="file" class="hidden"
           accept="image/png,image/jpeg,image/webp,image/gif"
           @change="onPick"/>
  </div>
</template>

<script>
export default {
  name: "MyAvatarDock",
  data: () => ({
    myUrl: localStorage.getItem("myPhotoUrl") || "",
    origin: "",
  }),
  computed:{
    srcOk(){
      const p = this.myUrl;
      if (!p) return "";
      if (/^https?:\/\//i.test(p)) return p;
      if (!this.origin) return "";
      const path = p.startsWith("/") ? p : `/${p}`;
      return this.origin + path;
    }
  },
  mounted(){
    // calcola l'origin in modo sicuro (senza rompere il render)
    try {
      const base = this.$axios?.defaults?.baseURL || "";
      this.origin = base ? new URL(base).origin : window.location.origin;
    } catch {
      this.origin = window.location.origin;
    }
  },
  methods:{
    pick(){ this.$refs.picker?.click(); },
    async onPick(e){
      const f = e.target.files?.[0];
      if (!f) return;
      const fd = new FormData(); fd.append("photo", f);
      try{
        const res = await this.$axios?.put?.("/me/photo", fd, {
          headers: { "Content-Type":"multipart/form-data" }
        });
        const url = res?.data?.url;
        if (url) {
          localStorage.setItem("myPhotoUrl", url);
          // aggiorna e forza refresh visivo
          this.myUrl = url + `?t=${Date.now()}`;
        } else {
          // se l'API non ritorna url, ricarica (pratica ma semplice)
          window.location.reload();
        }
      } catch (err){
        console.error(err);
        alert("Upload failed");
      } finally {
        e.target.value = "";
      }
    }
  }
}
</script>

<style scoped>
.me-dock{
  position: fixed; left: 16px; bottom: 16px;
  width: 56px; height: 56px; border-radius: 50%;
  border: 2px solid #111; background: #fff; overflow: hidden;
  display: grid; place-items: center; cursor: pointer; z-index: 2000;
}
.me-dock img{ width:100%; height:100%; object-fit:cover; display:block; }
.fallback{ font-weight: 800; }
.hidden{ display:none; }
</style>
