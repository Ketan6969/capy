const result = document.querySelector('.result');
if (result) {
    console.log("REACT RESULT: " + result.textContent);
} else {
    console.log("REACT RESULT: FAILED TO HYDRATE");
}
