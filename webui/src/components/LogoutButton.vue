<template>
  <button class="btn btn-outline-danger w-100" @click="logout">
    Logout
  </button>
</template>

<script>
export default {
  name: "LogoutButton",
  methods: {
    logout() {
      localStorage.removeItem("identifier");
      this.$router.replace("/");
      location.reload();
    }
  }
};
</script>

<!--
  methods: {
    logout() {
      localStorage.removeItem("identifier");
      this.$router.push("/"); // oppure "/login" se hai una route Login
    },

    <button class="link-mono link-mono--danger" @click="logout()">Logout</button>


-->