<script>
export default {
  name: "LoginView",
  data() { return { name: "", errormsg: null, loading: false }; },
  methods: {
    async login() {
      this.loading = true; this.errormsg = null;
      try {
        const { data } = await this.$axios.post("/v1/session", { name: this.name });
        localStorage.setItem("identifier", data.identifier);
        this.$router.push("/");   

      } catch (e) {
        this.errormsg = e?.response?.data?.message || e?.message || "Login failed";

      } finally {
        this.loading = false;
      }
    }
  }
};
</script>

<template>
  <div class="centered">
    <h1 class="h-mono mb-3">Login</h1>

    <div class="card-mono p-4 stack-24">
      <input
        v-model="name"
        class="form-control input-mono"
        type="text"
        placeholder="Username"
        minlength="3"
        maxlength="16"
        required
      />

      <button class="btn-mono" :disabled="loading" @click="login">
        {{ loading ? "..." : "Enter" }}
      </button>

      <p v-if="errormsg" class="text-danger mb-0">{{ errormsg }}</p>
      <p class="help mb-0">Use 3–16 characters. No password, just username.</p>
    </div>
  </div>
</template>
