<script>
import SidebarConversations from "@/views/SidebarConversations.vue";
import MyAvatarDock from "@/components/MyAvatarDock.vue";

export default {
  name: "ChatLayout",
  components: { SidebarConversations, MyAvatarDock },
};
</script>

<template>
  <div class="chat-shell">
    <aside class="chat-sidebar">
      <SidebarConversations />
    </aside>

    <main class="chat-main">
      <router-view />
    </main>

    <MyAvatarDock />
  </div>
</template>

<style scoped>
.chat-shell{ display:grid; grid-template-columns:360px 1fr; height:100vh; }
.chat-sidebar{ border-right:2px solid #111; overflow:auto; background:#fff; }
.chat-main{ overflow:auto; background:#fff; }
</style>
