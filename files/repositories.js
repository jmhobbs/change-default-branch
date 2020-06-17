$(function () {
  function refreshLabels (target) { 
    $("span.label").each(function () {
      if(this.innerText == "master") {
        this.classList.add("error");
        return
      }
      if(this.innerText == target) {
        this.classList.remove("warning");
        this.classList.add("success");
      } else {
        this.classList.remove("success");
        this.classList.add("warning");
      }
    });
  }

  $("input[name=default_branch]").on("change", function () {
    var target = $(this).val().trim();
    if(target.length == 0) {
      this.classList.add("error");
      return
    } else {
      this.classList.remove("error");
    }
    refreshLabels(target);
  });

  refreshLabels($("input[name=default_branch]").val());
});