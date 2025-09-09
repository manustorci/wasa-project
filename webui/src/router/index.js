import { createRouter, createWebHashHistory } from "vue-router";
import ChatLayout from "@/views/ChatLayout.vue";
import ConversationView from "@/views/ConversationView.vue";
import ConversationsEmpty from "@/views/ConversationsEmpty.vue";
import ConversationsView from "@/views/ConversationsView.vue";
import LoginView from "@/views/LoginView.vue";

const routes = [
  { path: "/", redirect: "/chat" },
  { path: "/login", component: LoginView },

  {
    path: "/chat",
    component: ChatLayout,
    children: [
      { path: "", component: ConversationsEmpty },
      { path: ":id", name: "conversation", component: ConversationView },
    ],
  },

  { path: "/conversations", component: ConversationsView },
  { path: "/conversations/:id", redirect: (to) => ({ path: `/chat/${to.params.id}` }) },
];

const router = createRouter({
  history: createWebHashHistory(),
  routes,
});

// auth guard
router.beforeEach((to, _from, next) => {
  const id = localStorage.getItem("identifier");
  if (!id && to.path !== "/login") return next("/login");
  if (id && to.path === "/login") return next("/chat");
  next();
});

export default router;
