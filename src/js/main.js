document.addEventListener('DOMContentLoaded', () => {
    // const titles = [];
    let index = 0;

//    function changeTitle() {
//        document.title = titles[index];
//        index = (index + 1) % titles.length;
//        setTimeout(changeTitle, 200);
//    }
//    changeTitle();

    const nextParticle = new NextParticle({
        image: document.getElementById("logo"),
        width: window.innerWidth,
        height: window.innerHeight * 0.8,
        maxWidth: Math.min(window.innerWidth * 0.8, 400),
        particleGap: 4,
        velocity: 0.5,
        proximity: 100,
        mouseForce: 200,
        color: "#FF0000", 
    });

    function resizeParticle() {
        nextParticle.width = window.innerWidth;
        nextParticle.height = window.innerHeight * 0.8;
        nextParticle.maxWidth = Math.min(window.innerWidth * 0.8, 400);
        nextParticle.start();
    }

    window.onresize = resizeParticle;
    resizeParticle();
});

