$(function () {
  var hadErrors = false;

  var csrfToken = $("meta[name=csrf-token]").attr("content");
  console.log(csrfToken);

  $(".collapsible .state").on("click", function () {
    $(this).parents(".collapsible").toggleClass("collapsed");
  });

  function processNext() {
    var next = $(".pending").first();
    if(next.length == 0) { 
      // All done!
      if(hadErrors) {
        $(".container h1").text("Done, with errors.");
      } else {
        $(".container h1").text("Success!");
      }
      return;
    }
    next.removeClass("pending").addClass("processing");
    next.find(".state").text("Processing...");
    $.ajax({
      type: "POST",
      url: "/repos/convert",
      data: {
        repository: next.data("repo"),
        branch: ChangeBranchData.Branch,
      },
      headers: {
        "X-CSRF-Token": csrfToken,
      },
      success: function (data, _, _) {
        next.removeClass("processing").addClass("complete");
        next.find(".state").text("Complete!");
        next.find("pre").text(data);
        setTimeout(processNext, 500);
      },
      error: function (xhr, _, _) {
        hadErrors = true;
        next.removeClass("processing").addClass("error");
        next.find(".state").text("Error");
        next.find("pre").text(xhr.responseText);
        next.find(".collapsible").removeClass("collapsed");
        setTimeout(processNext, 1500);
      },
    });
  }
  processNext();
});